package engine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// The benchmarks in this file exercise the engine dispatch path against the
// noop backend. The noop backend does no injection, so the numbers reflect the
// cost of the engine itself: submit synchronization, queue hand-off, key
// bookkeeping and the worker round trip.

// BenchmarkEngineSubmitSingle splits the press and release halves of the hot
// path. "press_held" keeps one key held across iterations (held-set map hit),
// while "release_unheld" releases a key the engine never saw (held-set miss).
func BenchmarkEngineSubmitSingle(b *testing.B) {
	e := newBenchEngine(b, DriveNoop, defaultDispatchQueueSize, 0)
	ctx := context.Background()
	held := KeyCodes{FakerInput: fakerInputKeyA}
	unheld := KeyCodes{FakerInput: fakerInputKeyB}

	if err := e.EnginePress(ctx, held); err != nil {
		b.Fatalf("pre-press: %v", err)
	}

	b.Run("press_held", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if err := e.EnginePress(ctx, held); err != nil {
				b.Fatalf("press: %v", err)
			}
		}
		reportThroughput(b, 1)
	})

	b.Run("release_unheld", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if err := e.EngineRelease(ctx, unheld); err != nil {
				b.Fatalf("release: %v", err)
			}
		}
		reportThroughput(b, 1)
	})
}

// BenchmarkEngineSubmitQueueSize shows how the dispatch buffer affects the hot
// path. Because submit waits for the worker, a larger queue only changes
// burst absorption and channel internals, not steady-state throughput.
func BenchmarkEngineSubmitQueueSize(b *testing.B) {
	key := KeyCodes{FakerInput: fakerInputKeyA}
	for _, size := range []int{1, 8, 1024, 65536} {
		b.Run(fmt.Sprintf("q%d", size), func(b *testing.B) {
			e := newBenchEngine(b, DriveNoop, size, 0)
			benchPressRelease(b, e, key)
		})
	}
}

// BenchmarkEngineKeyChord measures the per-key cost inside a single submit.
// Multiple keys in one operation map to a chord (all pressed, then all
// released).
func BenchmarkEngineKeyChord(b *testing.B) {
	keys := []KeyCodes{
		{FakerInput: fakerInputKeyW},
		{FakerInput: fakerInputKeyA},
		{FakerInput: fakerInputKeyS},
		{FakerInput: fakerInputKeyD},
	}
	for _, n := range []int{1, 2, 4} {
		b.Run(fmt.Sprintf("keys%d", n), func(b *testing.B) {
			e := newBenchEngine(b, DriveNoop, defaultDispatchQueueSize, 0)
			benchPressRelease(b, e, keys[:n]...)
		})
	}
}

// BenchmarkEngineEffectiveKeyRelease isolates the release-side held-set lookup.
// A release that matches a held entry exactly is an O(1) map hit; a release
// whose modifier flags differ falls back to an O(n) scan over heldKeys.
func BenchmarkEngineEffectiveKeyRelease(b *testing.B) {
	const maxHeld = 128
	heldKeys := make([]KeyCodes, maxHeld)
	for i := range heldKeys {
		heldKeys[i] = KeyCodes{FakerInput: fakerInputKeyA + byte(i)}
	}
	last := heldKeys[maxHeld-1]

	cases := []struct {
		name  string
		query KeyCodes
	}{
		{"exact_hit", heldKeys[0]},
		{"same_physical_scan", KeyCodes{FakerInput: last.FakerInput, Modifiers: ModLShift}},
		{"miss_scan", KeyCodes{FakerInput: 0xFF}},
	}

	for _, held := range []int{1, 32, 128} {
		for _, tc := range cases {
			b.Run(fmt.Sprintf("held%d/%s", held, tc.name), func(b *testing.B) {
				e := NewEngine()
				for i := 0; i < held; i++ {
					e.heldKeys = append(e.heldKeys, heldKeys[i])
					e.heldKeySet[heldKeys[i]] = struct{}{}
				}

				query := tc.query
				if tc.name == "same_physical_scan" {
					// Query the last held key with different modifier flags so
					// the lookup has to scan the whole held slice and match by
					// physical key identity.
					query = KeyCodes{FakerInput: heldKeys[held-1].FakerInput, Modifiers: ModLShift}
				}

				if got := e.effectiveKey(query, false); got.FakerInput != query.FakerInput {
					b.Fatalf("effectiveKey(%v) = %v, want FakerInput 0x%02x", query, got, query.FakerInput)
				}

				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					benchSinkKey = e.effectiveKey(query, false)
				}
			})
		}
	}
}

// BenchmarkEngineLatencySweep models backend cost with NoopDevice's simulated
// latency. Steady-state throughput is bounded by 1/latency because a single
// worker serializes every event.
func BenchmarkEngineLatencySweep(b *testing.B) {
	key := KeyCodes{FakerInput: fakerInputKeyA}
	// Sub-millisecond sleeps are dominated by the Windows timer granularity on
	// some hosts (~0.5ms), so the meaningful part of this sweep starts at 1ms.
	for _, micros := range []int{0, 100, 1000, 2000, 5000} {
		b.Run(fmt.Sprintf("%dus", micros), func(b *testing.B) {
			e := newBenchEngine(b, DriveNoop, defaultDispatchQueueSize, micros)
			benchPressRelease(b, e, key)
		})
	}
}

// BenchmarkEngineKeyboardParallel measures concurrent submitters on the single
// keyboard worker.
func BenchmarkEngineKeyboardParallel(b *testing.B) {
	e := newBenchEngine(b, DriveNoop, defaultDispatchQueueSize, 0)
	ctx := context.Background()
	key := KeyCodes{FakerInput: fakerInputKeyA}
	var ops atomic.Int64

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if err := e.EnginePress(ctx, key); err != nil {
				b.Errorf("press: %v", err)
				return
			}
			if err := e.EngineRelease(ctx, key); err != nil {
				b.Errorf("release: %v", err)
				return
			}
			ops.Add(2)
		}
	})
	reportOpsPerSecond(b, ops.Load())
}

// BenchmarkEngineMouseParallel is the mouse-only counterpart. It should reach a
// throughput close to the keyboard-only benchmark, since they run on separate
// workers.
func BenchmarkEngineMouseParallel(b *testing.B) {
	e := newBenchEngine(b, DriveNoop, defaultDispatchQueueSize, 0)
	ctx := context.Background()
	var ops atomic.Int64

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if err := e.EngineMouseMove(ctx, 1, -1); err != nil {
				b.Errorf("mouse move: %v", err)
				return
			}
			ops.Add(1)
		}
	})
	reportOpsPerSecond(b, ops.Load())
}

// BenchmarkEngineMixedWorkload compares running keyboard and mouse work
// serially in one loop against running them concurrently. If the two engine
// workers are independent, the concurrent case is bounded by the slower
// (keyboard) half instead of the sum, so ops/s should be clearly higher.
func BenchmarkEngineMixedWorkload(b *testing.B) {
	e := newBenchEngine(b, DriveNoop, defaultDispatchQueueSize, 0)
	ctx := context.Background()
	key := KeyCodes{FakerInput: fakerInputKeyA}

	keyboard := func() error {
		if err := e.EnginePress(ctx, key); err != nil {
			return err
		}
		return e.EngineRelease(ctx, key)
	}

	cases := []struct {
		name string
		run  func(b *testing.B) int64
	}{
		{
			name: "serial",
			run: func(b *testing.B) int64 {
				for i := 0; i < b.N; i++ {
					if err := keyboard(); err != nil {
						b.Errorf("keyboard: %v", err)
						return 0
					}
					if err := e.EngineMouseMove(ctx, 1, -1); err != nil {
						b.Errorf("mouse move: %v", err)
						return 0
					}
				}
				return 3 * int64(b.N)
			},
		},
		{
			name: "concurrent",
			run: func(b *testing.B) int64 {
				// Both halves run b.N iterations, but on separate engine
				// workers, so the elapsed time is the slower half rather than
				// the sum.
				var wg sync.WaitGroup
				wg.Add(2)
				go func() {
					defer wg.Done()
					for i := 0; i < b.N; i++ {
						if err := keyboard(); err != nil {
							b.Errorf("keyboard: %v", err)
							return
						}
					}
				}()
				go func() {
					defer wg.Done()
					for i := 0; i < b.N; i++ {
						if err := e.EngineMouseMove(ctx, 1, -1); err != nil {
							b.Errorf("mouse move: %v", err)
							return
						}
					}
				}()
				wg.Wait()
				return 3 * int64(b.N)
			},
		},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			ops := tc.run(b)
			reportOpsPerSecond(b, ops)
		})
	}
}

// BenchmarkEngineMouseEvents covers each mouse event kind on a single worker.
func BenchmarkEngineMouseEvents(b *testing.B) {
	e := newBenchEngine(b, DriveNoop, defaultDispatchQueueSize, 0)
	ctx := context.Background()

	cases := []struct {
		name string
		run  func() error
	}{
		{"move_rel", func() error { return e.EngineMouseMove(ctx, 1, -1) }},
		{"move_abs", func() error { return e.EngineMouseMoveTo(ctx, 5000, 5000) }},
		{"button_down_held", func() error { return e.EngineMouseButtonDown(ctx, MouseButtonLeft) }},
		{"button_up_unheld", func() error { return e.EngineMouseButtonUp(ctx, MouseButtonRight) }},
		{"wheel", func() error { return e.EngineMouseWheel(ctx, 1) }},
		{"hwheel", func() error { return e.EngineMouseHWheel(ctx, -1) }},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := tc.run(); err != nil {
					b.Fatalf("%s: %v", tc.name, err)
				}
			}
			reportThroughput(b, 1)
		})
	}
}

// BenchmarkEngineCanceledContext measures the fast reject path. A request whose
// context is already canceled must return before allocating an op or touching
// the queue.
func BenchmarkEngineCanceledContext(b *testing.B) {
	e := newBenchEngine(b, DriveNoop, defaultDispatchQueueSize, 0)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := e.EnginePress(ctx, KeyCodes{FakerInput: fakerInputKeyA}); !errors.Is(err, context.Canceled) {
			b.Fatalf("press with canceled context: err=%v, want context.Canceled", err)
		}
	}
}

// BenchmarkEngineDrainQueue measures the shutdown drain path without a worker
// running: the queue is filled, then drained in one call. Each iteration
// processes a full queue.
func BenchmarkEngineDrainQueue(b *testing.B) {
	prevCfg := setBenchGlobalConfig(DriveNoop, defaultDispatchQueueSize, 0)
	defer restoreBenchGlobalConfig(prevCfg)

	e := NewEngine()
	e.driveName = DriveNoop
	e.driveClient = NewNoopDevice(0)
	ctx := context.Background()
	queue := cap(e.ops)
	fill := func() {
		for i := 0; i < queue; i++ {
			e.ops <- keyOp{
				ctx:       ctx,
				isPressed: true,
				keys:      []KeyCodes{{FakerInput: fakerInputKeyA}},
				resp:      make(chan error, 1),
			}
		}
	}

	// Allocation counts are intentionally not reported: the queue is refilled
	// while the timer is stopped, but the Go benchmark harness still counts
	// those allocations in B/op.
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		fill()
		b.StartTimer()
		e.drain()
	}
	b.StopTimer()
	if seconds := b.Elapsed().Seconds(); seconds > 0 {
		b.ReportMetric(float64(queue)*float64(b.N)/seconds, "ops/s")
	}
}

// BenchmarkEngineCloseWithHeldKeys measures CloseEngine's shutdown cost while
// the engine still holds keys, so it has to release every one of them. Setup
// (engine start plus the presses) is excluded from the measurement.
func BenchmarkEngineCloseWithHeldKeys(b *testing.B) {
	for _, held := range []int{1, 32} {
		b.Run(fmt.Sprintf("held%d", held), func(b *testing.B) {
			prevLogger := discardBenchLogs()
			// A small queue keeps per-engine setup cheap; close only needs to
			// drain whatever is in flight.
			prevCfg := setBenchGlobalConfig(DriveNoop, 8, 0)
			defer func() {
				restoreBenchGlobalConfig(prevCfg)
				slog.SetDefault(prevLogger)
			}()

			ctx := context.Background()
			keys := make([]KeyCodes, held)
			for i := range keys {
				keys[i] = KeyCodes{FakerInput: fakerInputKeyA + byte(i)}
			}

			// Allocation counts are intentionally not reported: engine setup
			// allocates while the timer is stopped, but the harness still counts
			// those allocations in B/op.
			for i := 0; i < b.N; i++ {
				e := NewEngine()
				if err := e.StartEngine(); err != nil {
					b.Fatalf("StartEngine: %v", err)
				}
				for _, key := range keys {
					if err := e.EnginePress(ctx, key); err != nil {
						b.Fatalf("press: %v", err)
					}
				}

				b.StartTimer()
				err := e.CloseEngine()
				b.StopTimer()
				if err != nil {
					b.Fatalf("CloseEngine: %v", err)
				}
			}
			if seconds := b.Elapsed().Seconds(); seconds > 0 {
				b.ReportMetric(float64(held)*float64(b.N)/seconds, "keys/s")
			}
		})
	}
}

// BenchmarkEngineLatencyDistribution records every round trip and reports
// p50/p90/p99/max alongside ns/op. The two time.Now calls add a small constant
// overhead to each sample, so use it for shape, not absolute values.
func BenchmarkEngineLatencyDistribution(b *testing.B) {
	e := newBenchEngine(b, DriveNoop, defaultDispatchQueueSize, 0)
	ctx := context.Background()
	key := KeyCodes{FakerInput: fakerInputKeyA}
	samples := make([]time.Duration, b.N)

	freq := benchQPCTicksPerSecond()
	if freq <= 0 {
		b.Fatal("QueryPerformanceFrequency failed")
	}
	nsPerTick := float64(time.Second) / float64(freq)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		start := benchQPCNow()
		if err := e.EnginePress(ctx, key); err != nil {
			b.Fatalf("press: %v", err)
		}
		if err := e.EngineRelease(ctx, key); err != nil {
			b.Fatalf("release: %v", err)
		}
		samples[i] = time.Duration(float64(benchQPCNow()-start) * nsPerTick)
	}
	b.StopTimer()
	reportPercentiles(b, samples)
}
