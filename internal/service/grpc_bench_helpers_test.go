package service

import (
	"log/slog"
	"testing"

	"GoKeyMux/internal/config"
	"GoKeyMux/internal/engine"
)

// newGRPCBenchEngine starts a noop-backed engine for the gRPC benchmarks. It
// mirrors the engine package's benchmark helper without importing test-only
// code across packages.
func newGRPCBenchEngine(b *testing.B) *engine.Engine {
	b.Helper()

	prevLogger := slog.Default()
	slog.SetDefault(slog.New(slog.DiscardHandler))

	prevDrive := config.GlobalConfig.EnabledDriveName
	prevQueue := config.GlobalConfig.DispatchQueueSize
	prevLatency := config.GlobalConfig.NoopLatencyMicros
	config.GlobalConfig.EnabledDriveName = config.DriveNoop
	config.GlobalConfig.DispatchQueueSize = 1024
	config.GlobalConfig.NoopLatencyMicros = 0

	e := engine.NewEngine()
	if err := e.StartEngine(); err != nil {
		config.GlobalConfig.EnabledDriveName = prevDrive
		config.GlobalConfig.DispatchQueueSize = prevQueue
		config.GlobalConfig.NoopLatencyMicros = prevLatency
		slog.SetDefault(prevLogger)
		b.Fatalf("StartEngine: %v", err)
	}

	b.Cleanup(func() {
		_ = e.CloseEngine()
		config.GlobalConfig.EnabledDriveName = prevDrive
		config.GlobalConfig.DispatchQueueSize = prevQueue
		config.GlobalConfig.NoopLatencyMicros = prevLatency
		slog.SetDefault(prevLogger)
	})
	return e
}

// reportThroughput reports derived engine operations per second for the gRPC
// benchmarks (each iteration performs opsPerIteration engine submissions).
func reportThroughput(b *testing.B, opsPerIteration int) {
	seconds := b.Elapsed().Seconds()
	if seconds <= 0 {
		return
	}
	b.ReportMetric(float64(b.N)*float64(opsPerIteration)/seconds, "ops/s")
}
