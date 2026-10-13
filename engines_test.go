package main

import (
	"context"
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

	// The worker reads the drive name at dispatch time, so switching to an
	// unknown drive makes apply return an error that must reach the submitter.
	GlobalConfig.EnabledDriveName = DriveName("bogus")
	if err := e.EnginePress(context.Background(), KeyCodes{}); err == nil {
		t.Fatal("expected error propagated from worker")
	}
}

func TestEngineApplyUnsupportedDrive(t *testing.T) {
	prev := GlobalConfig.EnabledDriveName
	GlobalConfig.EnabledDriveName = DriveName("bogus")
	t.Cleanup(func() { GlobalConfig.EnabledDriveName = prev })

	e := NewEngine()
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
	prev := GlobalConfig.EnabledDriveName
	t.Cleanup(func() { GlobalConfig.EnabledDriveName = prev })

	e := NewEngine()
	for _, drive := range []DriveName{DriveWinputWithWindow, DriveWinputWithInterception} {
		GlobalConfig.EnabledDriveName = drive
		if err := e.applyMouse(context.Background(), mouseEvent{kind: mouseEventMoveRel, dx: 1, dy: 1}); err == nil {
			t.Errorf("expected error for drive %s", drive)
		}
	}
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
