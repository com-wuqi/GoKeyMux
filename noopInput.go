package main

import (
	"context"
	"fmt"
	"time"
)

// NoopDevice is a drive backend that performs no real input injection. It is
// used to load-test the gRPC/service layer (serialization, key resolution,
// dispatch) at high volume without side effects. It optionally simulates a
// fixed per-event latency to model driver cost.
//
// pressCount/releaseCount and the mouse counters are only mutated by the
// single Engine worker goroutine (keyboard) and the single Engine mouse worker
// goroutine respectively, so they need no synchronization; the stats must be
// read when no concurrent writes are in flight (after drain/shutdown, or from
// tests).
type NoopDevice struct {
	pressCount   uint64
	releaseCount uint64
	latency      time.Duration

	mouseMoveCount     uint64
	mouseMoveAbsCount  uint64
	mouseButtonDownCnt uint64
	mouseButtonUpCnt   uint64
	mouseWheelCount    uint64
	mouseHWheelCount   uint64
}

func NewNoopDevice(latency time.Duration) *NoopDevice {
	return &NoopDevice{latency: latency}
}

func (d *NoopDevice) Press(ctx context.Context, _ KeyCodes) error {
	if err := d.applyLatency(ctx); err != nil {
		return err
	}
	d.pressCount++
	return nil
}

func (d *NoopDevice) Release(ctx context.Context, _ KeyCodes) error {
	if err := d.applyLatency(ctx); err != nil {
		return err
	}
	d.releaseCount++
	return nil
}

// Mouse processes a single backend-neutral mouse event, incrementing the
// corresponding counter. It performs no real injection.
func (d *NoopDevice) Mouse(ctx context.Context, ev mouseEvent) error {
	if err := d.applyLatency(ctx); err != nil {
		return err
	}
	switch ev.kind {
	case mouseEventMoveRel:
		d.mouseMoveCount++
	case mouseEventMoveAbs:
		d.mouseMoveAbsCount++
	case mouseEventButton:
		if ev.isPressed {
			d.mouseButtonDownCnt++
		} else {
			d.mouseButtonUpCnt++
		}
	case mouseEventWheel:
		d.mouseWheelCount++
	case mouseEventHWheel:
		d.mouseHWheelCount++
	default:
		return fmt.Errorf("unknown mouse event %d", ev.kind)
	}
	return nil
}

// applyLatency simulates per-event driver cost. It honors ctx cancellation so
// an aborted request stops waiting instead of blocking on an uninterruptible
// sleep.
func (d *NoopDevice) applyLatency(ctx context.Context) error {
	if d.latency <= 0 {
		return nil
	}
	timer := time.NewTimer(d.latency)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (d *NoopDevice) Stats() (press, release uint64) {
	return d.pressCount, d.releaseCount
}

// MouseStats returns the accumulated mouse event counters.
func (d *NoopDevice) MouseStats() (move, moveAbs, buttonDown, buttonUp, wheel, hwheel uint64) {
	return d.mouseMoveCount, d.mouseMoveAbsCount, d.mouseButtonDownCnt, d.mouseButtonUpCnt, d.mouseWheelCount, d.mouseHWheelCount
}
