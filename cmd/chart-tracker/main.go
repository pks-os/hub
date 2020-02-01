package main

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/rs/zerolog/log"
	"github.com/tegioz/hub/internal/hub"
	"github.com/tegioz/hub/internal/util"
)

func main() {
	// Setup configuration and logger
	cfg, err := util.SetupConfig("chart-tracker")
	if err != nil {
		log.Fatal().Err(err).Msg("Configuration setup failed")
	}
	fields := map[string]interface{}{
		"cmd": "chart-tracker",
	}
	if err := util.SetupLogger(cfg, fields); err != nil {
		log.Fatal().Err(err).Msg("Logger setup failed")
	}

	// Shutdown gracefully when SIGINT or SIGTERM signal is received
	log.Info().Int("pid", os.Getpid()).Msg("Chart tracker started")
	ctx, cancel := context.WithCancel(context.Background())
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-shutdown
		cancel()
		log.Info().Msg("Chart tracker shutting down..")
	}()

	// Setup database and hub api
	db, err := util.SetupDB(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Database setup failed")
	}
	hubApi := hub.New(db)

	// Launch dispatcher and workers and wait for them to finish
	var wg sync.WaitGroup
	dispatcher := newDispatcher(ctx, hubApi)
	wg.Add(1)
	go dispatcher.run(&wg, cfg.GetStringSlice("tracker.repositoriesNames"))
	for i := 0; i < cfg.GetInt("tracker.numWorkers"); i++ {
		w := newWorker(ctx, i, hubApi)
		wg.Add(1)
		go w.run(&wg, dispatcher.Queue)
	}
	wg.Wait()
	log.Info().Msg("Chart tracker finished")
}