package main

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/middleware"
	"github.com/kkz6/launch-go/internal/pkg/console"
)

func (w *workerRuntime) run(ctx console.Context) error {
	mux := asynq.NewServeMux()
	w.kernel.BootJobs(mux)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(quit)

	go w.runWorker(ctx, mux)
	go w.runScheduler()

	<-quit
	w.shutdown(ctx)
	return nil
}

func (w *workerRuntime) runWorker(ctx console.Context, mux *asynq.ServeMux) {
	ctx.Info("Worker started")
	if err := w.server.Run(mux); err != nil {
		w.logger.Fatal().Err(err).Msg("Failed to start worker")
	}
}

func (w *workerRuntime) runScheduler() {
	w.logger.Info().Int("scheduled_tasks", w.scheduledTaskCount).Msg("Scheduler started")
	if err := w.scheduler.Run(); err != nil {
		w.logger.Error().Err(err).Msg("Scheduler error")
	}
}

func (w *workerRuntime) shutdown(ctx console.Context) {
	ctx.Info("Shutting down worker...")
	w.server.Shutdown()
	w.scheduler.Shutdown()
	w.kernel.Shutdown()
	w.broadcaster.Close()
	w.queueClient.Close()

	if w.sentryEnabled {
		middleware.FlushSentry(5 * time.Second)
	}
	ctx.Info("Worker stopped")
}
