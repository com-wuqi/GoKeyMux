//go:build integration

package main

import (
	"testing"
	"time"
)

func TestFakerInputIntegration(t *testing.T) {
	dev, err := FakerInputInit()
	if err != nil {
		t.Skipf("FakerInput device not available: %v", err)
	}
	defer dev.Close()

	version, err := dev.CheckAPIVersion()
	if err != nil {
		t.Fatalf("CheckAPIVersion() failed: %v", err)
	}
	t.Logf("CheckAPIVersion OK (version=%d)", version)

	if err := dev.Tap(fakerInputKeyA, 0); err != nil {
		t.Fatalf("Tap(fakerInputKeyA) failed: %v", err)
	}
	t.Log("Tap(fakerInputKeyA) OK")

	time.Sleep(100 * time.Millisecond)

	if err := dev.TypeText("Hello from FakerInput!"); err != nil {
		t.Fatalf("TypeText() failed: %v", err)
	}
	t.Log("TypeText OK")
}
