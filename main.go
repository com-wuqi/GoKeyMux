package main

import (
	"log/slog"
	"os"
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

	server, serverErrCh, err := StartService()
	if err != nil {
		slog.Error("Error starting service", "err", err)
		return
	}

	// clean up
	server.GracefulStop()
	err, ok := <-serverErrCh
	if ok {
		slog.Warn("Error stopping service", "err", err)
	}

}
