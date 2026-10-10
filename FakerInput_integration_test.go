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

func TestFakerInputMouseIntegration(t *testing.T) {
	dev, err := FakerInputInit()
	if err != nil {
		t.Skipf("FakerInput device not available: %v", err)
	}
	defer dev.Close()

	if err := dev.MouseMove(10, 10); err != nil {
		t.Fatalf("MouseMove(10,10) failed: %v", err)
	}
	t.Log("MouseMove OK")

	time.Sleep(50 * time.Millisecond)

	if err := dev.MouseButtonDown(fakerInputMouseButtonLeft); err != nil {
		t.Fatalf("MouseButtonDown(left) failed: %v", err)
	}
	time.Sleep(30 * time.Millisecond)
	if err := dev.MouseButtonUp(fakerInputMouseButtonLeft); err != nil {
		t.Fatalf("MouseButtonUp(left) failed: %v", err)
	}
	t.Log("Mouse click OK")

	time.Sleep(50 * time.Millisecond)

	if err := dev.MouseWheel(1); err != nil {
		t.Fatalf("MouseWheel(1) failed: %v", err)
	}
	t.Log("MouseWheel OK")

	// Simultaneous keyboard + mouse: hold a key while moving and clicking, to
	// exercise the independent keyboard/mouse report streams through the same
	// control handle.
	if err := dev.keyDown(fakerInputKeyA, 0); err != nil {
		t.Fatalf("keyDown(a) failed: %v", err)
	}
	if err := dev.MouseMove(5, 5); err != nil {
		t.Fatalf("MouseMove during key hold failed: %v", err)
	}
	if err := dev.MouseButtonDown(fakerInputMouseButtonRight); err != nil {
		t.Fatalf("MouseButtonDown(right) during key hold failed: %v", err)
	}
	time.Sleep(30 * time.Millisecond)
	if err := dev.MouseButtonUp(fakerInputMouseButtonRight); err != nil {
		t.Fatalf("MouseButtonUp(right) failed: %v", err)
	}
	if err := dev.keyUp(fakerInputKeyA, 0); err != nil {
		t.Fatalf("keyUp(a) failed: %v", err)
	}
	t.Log("Simultaneous keyboard + mouse OK")
}
