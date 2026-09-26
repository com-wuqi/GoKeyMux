package main

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNoopDeviceLatencyHonorsCancellation(t *testing.T) {
	d := NewNoopDevice(time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	if err := d.Press(ctx, KeyCodes{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Press with canceled ctx: got err %v, want context.Canceled", err)
	}
	if elapsed := time.Since(start); elapsed >= 100*time.Millisecond {
		t.Fatalf("Press should return promptly on cancellation, took %v", elapsed)
	}

	press, release := d.Stats()
	if press != 0 || release != 0 {
		t.Fatalf("Stats after canceled Press: press=%d release=%d, want 0/0", press, release)
	}

	if err := d.Release(ctx, KeyCodes{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Release with canceled ctx: got err %v, want context.Canceled", err)
	}
}

func TestNoopDevicePressRelease(t *testing.T) {
	d := NewNoopDevice(0)

	if err := d.Press(context.Background(), KeyCodes{}); err != nil {
		t.Fatalf("Press: %v", err)
	}
	if err := d.Release(context.Background(), KeyCodes{}); err != nil {
		t.Fatalf("Release: %v", err)
	}

	press, release := d.Stats()
	if press != 1 || release != 1 {
		t.Fatalf("Stats: press=%d release=%d, want 1/1", press, release)
	}
}
