package main

import (
	"sync/atomic"
	"time"
)

// NoopDevice is a drive backend that performs no real input injection. It is
// used to load-test the gRPC/service layer (serialization, key resolution,
// dispatch) at high volume without side effects. It optionally simulates a
// fixed per-event latency to model driver cost.
type NoopDevice struct {
	pressCount   atomic.Uint64
	releaseCount atomic.Uint64
	latency      time.Duration
}

func NewNoopDevice(latency time.Duration) *NoopDevice {
	return &NoopDevice{latency: latency}
}

func (d *NoopDevice) Press(KeyCodes) error {
	d.applyLatency()
	d.pressCount.Add(1)
	return nil
}

func (d *NoopDevice) Release(KeyCodes) error {
	d.applyLatency()
	d.releaseCount.Add(1)
	return nil
}

func (d *NoopDevice) applyLatency() {
	if d.latency > 0 {
		time.Sleep(d.latency)
	}
}

func (d *NoopDevice) Stats() (press, release uint64) {
	return d.pressCount.Load(), d.releaseCount.Load()
}
