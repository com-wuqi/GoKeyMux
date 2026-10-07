package main

import (
	"context"
	"time"
)

// NoopDevice is a drive backend that performs no real input injection. It is
// used to load-test the gRPC/service layer (serialization, key resolution,
// dispatch) at high volume without side effects. It optionally simulates a
// fixed per-event latency to model driver cost.
//
// pressCount/releaseCount are only mutated by the single Engine worker
// goroutine, so they need no synchronization; Stats must be read when no
// concurrent writes are in flight (after drain/shutdown, or from tests).
type NoopDevice struct {
	pressCount   uint64
	releaseCount uint64
	latency      time.Duration
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
