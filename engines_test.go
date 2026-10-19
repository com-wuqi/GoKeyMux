package main

import (
	"context"
	"errors"
	"sync"
	"testing"
)

// newNoopTestEngine starts an engine on the noop backend with a small dispatch
// queue, restoring global config afterwards.
func newNoopTestEngine(t *testing.T) *Engine {
	t.Helper()
	prevDrive := GlobalConfig.EnabledDriveName
	prevQueue := GlobalConfig.DispatchQueueSize
	prevLatency := GlobalConfig.NoopLatencyMicros
	GlobalConfig.EnabledDriveName = DriveNoop
	GlobalConfig.DispatchQueueSize = 8
	GlobalConfig.NoopLatencyMicros = 0
	e := NewEngine()
	if err := e.StartEngine(); err != nil {
		t.Fatalf("StartEngine: %v", err)
	}
	t.Cleanup(func() {
		e.CloseEngine()
		GlobalConfig.EnabledDriveName = prevDrive
		GlobalConfig.DispatchQueueSize = prevQueue
		GlobalConfig.NoopLatencyMicros = prevLatency
	})
	return e
}

func TestEngineConcurrentSubmitNoop(t *testing.T) {
	e := newNoopTestEngine(t)

	const n = 1000
	var wg sync.WaitGroup
	for range n {
		wg.Add(2)
		go func() {
			defer wg.Done()
			if err := e.EnginePress(context.Background(), KeyCodes{}); err != nil {
				t.Errorf("press: %v", err)
			}
		}()
		go func() {
			defer wg.Done()
			if err := e.EngineRelease(context.Background(), KeyCodes{}); err != nil {
				t.Errorf("release: %v", err)
			}
		}()
	}
	wg.Wait()

	dev, ok := e.driveClient.(*NoopDevice)
	if !ok {
		t.Fatalf("driveClient is not *NoopDevice, got %T", e.driveClient)
	}
	press, release := dev.Stats()
	if press != n || release != n {
		t.Fatalf("press=%d release=%d, want %d/%d", press, release, n, n)
	}
}

func TestEngineSubmitCancelledContext(t *testing.T) {
	e := newNoopTestEngine(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := e.EnginePress(ctx, KeyCodes{}); err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func TestEngineSubmitPropagatesError(t *testing.T) {
	e := newNoopTestEngine(t)

	// Closing the backend directly makes the worker's apply return an error
	// that must reach the submitter.
	dev, ok := e.driveClient.(*NoopDevice)
	if !ok {
		t.Fatalf("driveClient is not *NoopDevice, got %T", e.driveClient)
	}
	if err := dev.Close(); err != nil {
		t.Fatalf("close noop device: %v", err)
	}
	if err := e.EnginePress(context.Background(), KeyCodes{}); err == nil {
		t.Fatal("expected error propagated from worker")
	}
}

func TestEngineApplyUnsupportedDrive(t *testing.T) {
	e := NewEngine()
	e.driveName = DriveName("bogus")
	if err := e.apply(context.Background(), []KeyCodes{{}}, true); err == nil {
		t.Fatal("expected error for unsupported drive")
	}
}

func TestEngineConcurrentMouseKeyboardNoop(t *testing.T) {
	e := newNoopTestEngine(t)

	const n = 1000
	var wg sync.WaitGroup
	for range n {
		wg.Add(4)
		go func() {
			defer wg.Done()
			if err := e.EnginePress(context.Background(), KeyCodes{}); err != nil {
				t.Errorf("press: %v", err)
			}
		}()
		go func() {
			defer wg.Done()
			if err := e.EngineRelease(context.Background(), KeyCodes{}); err != nil {
				t.Errorf("release: %v", err)
			}
		}()
		go func() {
			defer wg.Done()
			if err := e.EngineMouseMove(context.Background(), 1, 2); err != nil {
				t.Errorf("mouse move: %v", err)
			}
		}()
		go func() {
			defer wg.Done()
			if err := e.EngineMouseButtonDown(context.Background(), MouseButtonLeft); err != nil {
				t.Errorf("mouse down: %v", err)
			}
		}()
	}
	wg.Wait()

	dev, ok := e.driveClient.(*NoopDevice)
	if !ok {
		t.Fatalf("driveClient is not *NoopDevice, got %T", e.driveClient)
	}
	press, release := dev.Stats()
	if press != n || release != n {
		t.Fatalf("press=%d release=%d, want %d/%d", press, release, n, n)
	}
	move, _, buttonDown, _, _, _ := dev.MouseStats()
	if move != n || buttonDown != n {
		t.Fatalf("mouse move=%d buttonDown=%d, want %d/%d", move, buttonDown, n, n)
	}
}

func TestEngineMouseApplyUnsupportedDrive(t *testing.T) {
	e := NewEngine()
	for _, drive := range []DriveName{DriveWinputWithWindow, DriveWinputWithInterception} {
		e.driveName = drive
		if err := e.applyMouse(context.Background(), mouseEvent{kind: mouseEventMoveRel, dx: 1, dy: 1}); err == nil {
			t.Errorf("expected error for drive %s", drive)
		}
	}
}

// TestCloseEngineReleasesHeldInput verifies that a graceful shutdown releases
// every key and mouse button that was pressed but never released.
func TestCloseEngineReleasesHeldInput(t *testing.T) {
	e := newNoopTestEngine(t)
	ctx := context.Background()
	if err := e.EnginePress(ctx, KeyCodes{FakerInput: fakerInputKeyA}); err != nil {
		t.Fatalf("press a: %v", err)
	}
	if err := e.EnginePress(ctx, KeyCodes{FakerInput: fakerInputKeyB}); err != nil {
		t.Fatalf("press b: %v", err)
	}
	if err := e.EngineMouseButtonDown(ctx, MouseButtonLeft); err != nil {
		t.Fatalf("mouse down: %v", err)
	}

	if err := e.CloseEngine(); err != nil {
		t.Fatalf("CloseEngine: %v", err)
	}

	dev, ok := e.driveClient.(*NoopDevice)
	if !ok {
		t.Fatalf("driveClient is not *NoopDevice, got %T", e.driveClient)
	}
	press, release := dev.Stats()
	if press != 2 || release != 2 {
		t.Fatalf("press=%d release=%d, want 2/2", press, release)
	}
	_, _, buttonDown, buttonUp, _, _ := dev.MouseStats()
	if buttonDown != 1 || buttonUp != 1 {
		t.Fatalf("buttonDown=%d buttonUp=%d, want 1/1", buttonDown, buttonUp)
	}
}

// TestCloseEngineReleasesBeforeBackendClose proves the release happens while the
// backend is still usable: the Noop backend rejects writes after Close, so the
// release counter can only reach 1 if it ran before CloseEngine closed it.
func TestCloseEngineReleasesBeforeBackendClose(t *testing.T) {
	e := newNoopTestEngine(t)
	if err := e.EnginePress(context.Background(), KeyCodes{FakerInput: fakerInputKeyA}); err != nil {
		t.Fatalf("press: %v", err)
	}
	dev, ok := e.driveClient.(*NoopDevice)
	if !ok {
		t.Fatalf("driveClient is not *NoopDevice, got %T", e.driveClient)
	}

	if err := e.CloseEngine(); err != nil {
		t.Fatalf("CloseEngine: %v", err)
	}
	if !dev.Closed() {
		t.Fatal("backend was not closed")
	}
	press, release := dev.Stats()
	if press != 1 || release != 1 {
		t.Fatalf("press=%d release=%d, want 1/1", press, release)
	}
	if err := dev.Release(context.Background(), KeyCodes{}); err == nil {
		t.Fatal("expected writes to a closed noop device to fail")
	}
}

func TestCloseEngineIdempotent(t *testing.T) {
	e := newNoopTestEngine(t)
	if err := e.CloseEngine(); err != nil {
		t.Fatalf("first CloseEngine: %v", err)
	}
	if err := e.CloseEngine(); err != nil {
		t.Fatalf("second CloseEngine: %v", err)
	}
}

func TestEngineRejectsSubmitAfterClose(t *testing.T) {
	e := newNoopTestEngine(t)
	if err := e.CloseEngine(); err != nil {
		t.Fatalf("CloseEngine: %v", err)
	}
	if err := e.EnginePress(context.Background(), KeyCodes{}); !errors.Is(err, ErrEngineClosed) {
		t.Fatalf("press after close: err=%v, want ErrEngineClosed", err)
	}
	if err := e.EngineMouseMove(context.Background(), 1, 1); !errors.Is(err, ErrEngineClosed) {
		t.Fatalf("mouse move after close: err=%v, want ErrEngineClosed", err)
	}
}

// TestDrainAppliesCanceledRelease verifies that a queued release whose request
// context has already been canceled is still applied while draining, using a
// detached context.
func TestDrainAppliesCanceledRelease(t *testing.T) {
	e := NewEngine()
	e.driveName = DriveNoop
	dev := NewNoopDevice(0)
	e.driveClient = dev

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	op := keyOp{
		ctx:       ctx,
		isPressed: false,
		keys:      []KeyCodes{{FakerInput: fakerInputKeyA}},
		resp:      make(chan error, 1),
	}
	e.ops <- op
	e.drain()

	if err := <-op.resp; err != nil {
		t.Fatalf("drain release: %v", err)
	}
	_, release := dev.Stats()
	if release != 1 {
		t.Fatalf("release=%d, want 1", release)
	}
}

// TestEngineConcurrentSubmitAndClose exercises the submit/close hand-off under
// the race detector: every submit must either succeed or return
// ErrEngineClosed, never touch a closed backend.
func TestEngineConcurrentSubmitAndClose(t *testing.T) {
	e := newNoopTestEngine(t)

	const n = 100
	var wg sync.WaitGroup
	for range n {
		wg.Go(func() {
			err := e.EnginePress(context.Background(), KeyCodes{FakerInput: fakerInputKeyA})
			if err != nil && !errors.Is(err, ErrEngineClosed) {
				t.Errorf("press: %v", err)
			}
		})
	}
	wg.Go(func() {
		if err := e.CloseEngine(); err != nil {
			t.Errorf("CloseEngine: %v", err)
		}
	})
	wg.Wait()
}

// TestEngineReleaseUsesHeldModifiers reproduces the "key was already released
// but shutdown still sends another release" case: pressing a shifted key and
// releasing its unshifted variant (or vice versa) identifies the same physical
// key but with a different KeyCodes.Modifiers value. The release must match the
// held entry and clear it; otherwise shutdown sends a spurious extra key-up and,
// for FakerInput, can leave the modifier held.
func TestEngineReleaseUsesHeldModifiers(t *testing.T) {
	e := newNoopTestEngine(t)
	ctx := context.Background()
	pressed := KeyCodes{FakerInput: fakerInputKeyA, Modifiers: ModLShift}
	released := KeyCodes{FakerInput: fakerInputKeyA}

	if err := e.EnginePress(ctx, pressed); err != nil {
		t.Fatalf("press: %v", err)
	}
	if err := e.EngineRelease(ctx, released); err != nil {
		t.Fatalf("release: %v", err)
	}
	if err := e.CloseEngine(); err != nil {
		t.Fatalf("CloseEngine: %v", err)
	}

	dev, ok := e.driveClient.(*NoopDevice)
	if !ok {
		t.Fatalf("driveClient is not *NoopDevice, got %T", e.driveClient)
	}
	press, release := dev.Stats()
	if press != 1 || release != 1 {
		t.Fatalf("press=%d release=%d, want 1/1 (the key was already released; shutdown must not release it again)", press, release)
	}
}

// BenchmarkEngineSubmitNoop measures the hot submit path against the noop
// backend (no injection cost), so changes to the submit-side synchronization or
// the held-input bookkeeping are visible.
func BenchmarkEngineSubmitNoop(b *testing.B) {
	prevDrive := GlobalConfig.EnabledDriveName
	prevQueue := GlobalConfig.DispatchQueueSize
	prevLatency := GlobalConfig.NoopLatencyMicros
	GlobalConfig.EnabledDriveName = DriveNoop
	GlobalConfig.DispatchQueueSize = defaultDispatchQueueSize
	GlobalConfig.NoopLatencyMicros = 0

	e := NewEngine()
	if err := e.StartEngine(); err != nil {
		b.Fatalf("StartEngine: %v", err)
	}
	b.Cleanup(func() {
		e.CloseEngine()
		GlobalConfig.EnabledDriveName = prevDrive
		GlobalConfig.DispatchQueueSize = prevQueue
		GlobalConfig.NoopLatencyMicros = prevLatency
	})

	ctx := context.Background()
	key := KeyCodes{FakerInput: fakerInputKeyA}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := e.EnginePress(ctx, key); err != nil {
			b.Fatalf("press: %v", err)
		}
		if err := e.EngineRelease(ctx, key); err != nil {
			b.Fatalf("release: %v", err)
		}
	}
}

// BenchmarkEngineSubmitNoopParallel measures concurrent submitters contending
// on the shared dispatch queue and the submit-side shutdown check.
func BenchmarkEngineSubmitNoopParallel(b *testing.B) {
	prevDrive := GlobalConfig.EnabledDriveName
	prevQueue := GlobalConfig.DispatchQueueSize
	prevLatency := GlobalConfig.NoopLatencyMicros
	GlobalConfig.EnabledDriveName = DriveNoop
	GlobalConfig.DispatchQueueSize = defaultDispatchQueueSize
	GlobalConfig.NoopLatencyMicros = 0

	e := NewEngine()
	if err := e.StartEngine(); err != nil {
		b.Fatalf("StartEngine: %v", err)
	}
	b.Cleanup(func() {
		e.CloseEngine()
		GlobalConfig.EnabledDriveName = prevDrive
		GlobalConfig.DispatchQueueSize = prevQueue
		GlobalConfig.NoopLatencyMicros = prevLatency
	})

	ctx := context.Background()
	key := KeyCodes{FakerInput: fakerInputKeyA}
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
		}
	})
}

func TestFakerInputAbsCoordinate(t *testing.T) {
	cases := []struct {
		in   int32
		want uint16
	}{
		{0, 0},
		{10000, 32767},
		{5000, 16383},
		{-1, 0},
		{10001, 32767},
	}
	for _, c := range cases {
		if got := FakerInputAbsCoordinate(c.in); got != c.want {
			t.Errorf("FakerInputAbsCoordinate(%d) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestFakerInputRelDelta(t *testing.T) {
	cases := []struct {
		in   int32
		want int16
	}{
		{0, 0},
		{1, 1},
		{-1, -1},
		{32767, 32767},
		{-32767, -32767},
		{32768, 32767},
		{-32768, -32767},
		{40000, 32767},
		{-40000, -32767},
	}
	for _, c := range cases {
		if got := FakerInputRelDelta(c.in); got != c.want {
			t.Errorf("FakerInputRelDelta(%d) = %d, want %d", c.in, got, c.want)
		}
	}
}
