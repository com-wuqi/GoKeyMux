package main

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	err := LoadConfig()
	if err != nil {
		slog.Error("Error loading config", "err", err)
		return
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: LogLevel2slogLevel(GlobalConfig.LogLevel),
	}))
	slog.SetDefault(logger)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	engine := NewEngine()
	err = engine.StartEngine()
	if err != nil {
		slog.Error("Error starting engine", "err", err)
		return
	}
	slog.Info("engine started")
	defer func(engine *Engine) {
		err := engine.CloseEngine()
		if err != nil {
			slog.Error("Error closing engine", "err", err)
		}
		slog.Info("engine closed")
	}(engine)

	slog.Info("server starting")
	server, serverErrCh, err := StartService(engine)
	if err != nil {
		slog.Error("Error starting service", "err", err)
		return
	}
	slog.Info("server started")
	defer func() {
		done := make(chan struct{})
		go func() { server.GracefulStop(); close(done) }()
		select {
		case <-done:
			slog.Info("server stopped")
		case <-time.After(time.Duration(GlobalConfig.GRPCServerShutdownTimeout) * time.Second):
			slog.Warn("Shutdown service timed out, use Stop")
			server.Stop()
		}
		if err, ok := <-serverErrCh; ok {
			slog.Warn("Error stopping service", "err", err)
		}
	}()

	<-sigCh
}
