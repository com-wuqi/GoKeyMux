package main

import (
	"GoKeyMux/version"
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	v, commit, buildTime := version.Info()
	slog.Info("GoKeyMux build info", "version", v, "commit", commit, "buildTime", buildTime)

	err := LoadConfig()
	if err != nil {
		slog.Error("Error loading config", "err", err)
		return
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: LogLevel2slogLevel(GlobalConfig.LogLevel),
	}))
	slog.SetDefault(logger)

	if GlobalConfig.PprofAddress != "" {
		go func() {
			slog.Info("pprof listening", "addr", GlobalConfig.PprofAddress)
			if err := http.ListenAndServe(GlobalConfig.PprofAddress, nil); err != nil {
				slog.Error("pprof server error", "err", err)
			}
		}()
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	engine := NewEngine()
	err = engine.StartEngine()
	if err != nil {
		slog.Error("Error starting engine", "err", err)
		return
	}
	slog.Info("engine started")

	// closeEngine runs the engine shutdown exactly once. It is called
	// explicitly on SIGINT/SIGTERM and deferred as a safety net for early
	// returns. The engine must be closed before the gRPC server so held input is
	// released while the backend is still usable, and so in-flight requests fail
	// fast with ErrEngineClosed instead of entering a queue that will not be
	// drained.
	engineClosed := false
	closeEngine := func() {
		if engineClosed {
			return
		}
		engineClosed = true
		if err := engine.CloseEngine(); err != nil {
			slog.Error("Error closing engine", "err", err)
		}
		slog.Info("engine closed")
	}
	defer closeEngine()

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

	// Release held input and reject new requests first; the deferred server
	// shutdown then terminates in-flight streams afterwards.
	closeEngine()
}
