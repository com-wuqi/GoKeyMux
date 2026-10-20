package engine

import (
	"context"
	"log/slog"
	"slices"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"GoKeyMux/internal/config"
)

// kernel32 is used for high-resolution latency sampling in benchmarks. The
// input package owns its own copy after the package split.
var kernel32 = syscall.NewLazyDLL("kernel32.dll")

// This file holds the shared plumbing for the benchmarks. All benchmarks in
// this package run sequentially (none of them calls b.Parallel), because they
// mutate process-wide state: config.GlobalConfig and the default slog logger.

// benchGlobalConfig is a snapshot of the config.GlobalConfig fields the benchmarks
// change. Restoring it keeps one benchmark from influencing the next.
type benchGlobalConfig struct {
	drive   DriveName
	queue   int
	latency int
}

func setBenchGlobalConfig(drive DriveName, queueSize, latencyMicros int) benchGlobalConfig {
	prev := benchGlobalConfig{
		drive:   config.GlobalConfig.EnabledDriveName,
		queue:   config.GlobalConfig.DispatchQueueSize,
		latency: config.GlobalConfig.NoopLatencyMicros,
	}
	config.GlobalConfig.EnabledDriveName = drive
	config.GlobalConfig.DispatchQueueSize = queueSize
	config.GlobalConfig.NoopLatencyMicros = latencyMicros
	return prev
}

func restoreBenchGlobalConfig(prev benchGlobalConfig) {
	config.GlobalConfig.EnabledDriveName = prev.drive
	config.GlobalConfig.DispatchQueueSize = prev.queue
	config.GlobalConfig.NoopLatencyMicros = prev.latency
}

// discardBenchLogs replaces the default logger so that shutdown warnings (for
// example one line per held key released by CloseEngine) do not pollute the
// benchmark output or introduce I/O jitter. The previous logger is returned so
// the caller can restore it.
func discardBenchLogs() *slog.Logger {
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.DiscardHandler))
	return prev
}

// newBenchEngine starts an engine for a benchmark and registers cleanup that
// closes it, restores config.GlobalConfig and restores the default logger. The noop
// backend is expected for everything except the realbackend benchmarks.
func newBenchEngine(b *testing.B, drive DriveName, queueSize, latencyMicros int) *Engine {
	b.Helper()
	prevLogger := discardBenchLogs()
	prevCfg := setBenchGlobalConfig(drive, queueSize, latencyMicros)

	e := NewEngine()
	if err := e.StartEngine(); err != nil {
		restoreBenchGlobalConfig(prevCfg)
		slog.SetDefault(prevLogger)
		b.Fatalf("StartEngine(%s): %v", drive, err)
	}
	b.Cleanup(func() {
		// CloseEngine is idempotent, so benchmarks that close the engine
		// themselves (shutdown benchmarks) are unaffected.
		_ = e.CloseEngine()
		restoreBenchGlobalConfig(prevCfg)
		slog.SetDefault(prevLogger)
	})
	return e
}

// benchPressRelease is the canonical engine workload: one press followed by one
// release of the same key set, repeated b.N times. It reports throughput in
// engine operations per second (two per iteration).
func benchPressRelease(b *testing.B, e *Engine, keys ...KeyCodes) {
	b.Helper()
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := e.EnginePress(ctx, keys...); err != nil {
			b.Fatalf("press: %v", err)
		}
		if err := e.EngineRelease(ctx, keys...); err != nil {
			b.Fatalf("release: %v", err)
		}
	}
	reportThroughput(b, 2)
}

// reportThroughput reports derived operations per second. Go's default ns/op
// metric does not make the serialization ceiling obvious for multi-op
// iterations, so this makes it explicit.
func reportThroughput(b *testing.B, opsPerIteration int) {
	b.Helper()
	seconds := b.Elapsed().Seconds()
	if seconds <= 0 {
		return
	}
	b.ReportMetric(float64(b.N)*float64(opsPerIteration)/seconds, "ops/s")
}

// reportOpsPerSecond reports an explicitly counted number of operations, for
// benchmarks where different workers perform different amounts of work per
// iteration.
func reportOpsPerSecond(b *testing.B, ops int64) {
	b.Helper()
	seconds := b.Elapsed().Seconds()
	if seconds <= 0 {
		return
	}
	b.ReportMetric(float64(ops)/seconds, "ops/s")
}

// reportPercentiles adds p50/p90/p99/max latency metrics (in nanoseconds) to a
// benchmark. It sorts samples in place, so callers must pass a slice they own
// and must call it after the timed loop has finished.
func reportPercentiles(b *testing.B, samples []time.Duration) {
	b.Helper()
	if len(samples) == 0 {
		return
	}
	slices.Sort(samples)
	at := func(q float64) time.Duration {
		idx := int(q * float64(len(samples)))
		if idx >= len(samples) {
			idx = len(samples) - 1
		}
		return samples[idx]
	}
	b.ReportMetric(float64(at(0.50).Nanoseconds()), "p50-ns")
	b.ReportMetric(float64(at(0.90).Nanoseconds()), "p90-ns")
	b.ReportMetric(float64(at(0.99).Nanoseconds()), "p99-ns")
	b.ReportMetric(float64(samples[len(samples)-1].Nanoseconds()), "max-ns")
}

// benchSinkKey prevents the compiler from eliminating micro-benchmarks whose
// result is otherwise unused.
var benchSinkKey KeyCodes

// Windows QueryPerformanceCounter is used for per-operation latency samples.
// time.Now is not good enough here: on some Windows hosts (including RDP/VM
// sessions) its granularity is ~0.5ms, so a microsecond-scale round trip is
// either measured as 0 or as a full clock tick. QPC typically ticks at 10MHz,
// giving ~100ns resolution.
//
// kernel32 is declared in winputWithInterception.go; the project is
// Windows-only, so reusing it keeps the benchmark dependency-free.
var (
	procQueryPerformanceCounter   = kernel32.NewProc("QueryPerformanceCounter")
	procQueryPerformanceFrequency = kernel32.NewProc("QueryPerformanceFrequency")
)

func benchQPCTicksPerSecond() int64 {
	var freq int64
	_, _, _ = procQueryPerformanceFrequency.Call(uintptr(unsafe.Pointer(&freq)))
	return freq
}

func benchQPCNow() int64 {
	var ticks int64
	_, _, _ = procQueryPerformanceCounter.Call(uintptr(unsafe.Pointer(&ticks)))
	return ticks
}
