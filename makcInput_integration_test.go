//go:build integration

package main

import (
	"context"
	"testing"
	"time"

	"github.com/aiwaki/makc"
)

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// TestMakcEngineIntegration drives the real makc backend through the Engine to
// verify the new mouse dispatch wiring and confirm the keyboard path still
// dispatches. The default auto backend uses InjectKeyboardInput/InjectMouseInput
// which do not update GetAsyncKeyState, so state is not asserted here (see
// TestMakcInputSendInputIntegration); success is a nil error from every call.
func TestMakcEngineIntegration(t *testing.T) {
	ctx := context.Background()

	prev := GlobalConfig.EnabledDriveName
	GlobalConfig.EnabledDriveName = DriveMakc
	t.Cleanup(func() { GlobalConfig.EnabledDriveName = prev })

	e := NewEngine()
	if err := e.StartEngine(); err != nil {
		t.Skipf("makc backend unavailable: %v", err)
	}
	t.Cleanup(func() { _ = e.CloseEngine() })

	client, ok := e.driveClient.(*makc.Client)
	if !ok {
		t.Fatalf("driveClient is not *makc.Client: %T", e.driveClient)
	}
	t.Logf("keyboard backend=%s mouse backend=%s",
		client.Keyboard.InjectionBackend(), client.Mouse.InjectionBackend())

	// Screen size must be cached at startup.
	if makcScreenSize.X <= 0 || makcScreenSize.Y <= 0 {
		t.Fatalf("screen size not cached: %+v", makcScreenSize)
	}
	t.Logf("cached screen size: %dx%d", makcScreenSize.X, makcScreenSize.Y)

	// Keyboard must still dispatch through the engine.
	kc, ok := KeyCodesFromName("shift")
	if !ok {
		t.Fatal("KeyCodesFromName(shift) failed")
	}
	if err := e.EnginePress(ctx, kc); err != nil {
		t.Fatalf("engine keyboard press: %v", err)
	}
	if err := e.EngineRelease(ctx, kc); err != nil {
		t.Fatalf("engine keyboard release: %v", err)
	}

	// Mouse dispatch must not error for any event kind.
	if err := e.EngineMouseMove(ctx, 10, 0); err != nil {
		t.Fatalf("engine mouse move: %v", err)
	}
	if err := e.EngineMouseMoveTo(ctx, 5000, 5000); err != nil {
		t.Fatalf("engine mouse moveTo: %v", err)
	}
	if err := e.EngineMouseButtonDown(ctx, MouseButtonLeft); err != nil {
		t.Fatalf("engine mouse button down: %v", err)
	}
	if err := e.EngineMouseButtonUp(ctx, MouseButtonLeft); err != nil {
		t.Fatalf("engine mouse button up: %v", err)
	}
	if err := e.EngineMouseWheel(ctx, 1); err != nil {
		t.Fatalf("engine mouse wheel: %v", err)
	}
	if err := e.EngineMouseHWheel(ctx, 1); err != nil {
		t.Fatalf("engine mouse hwheel: %v", err)
	}

	t.Log("makc engine keyboard+mouse wiring OK")
}

// TestMakcInputSendInputIntegration exercises the makc helper functions directly
// against a SendInput backend, which (unlike InjectKeyboardInput/InjectMouseInput)
// updates GetAsyncKeyState, so button and key state can be asserted.
func TestMakcInputSendInputIntegration(t *testing.T) {
	ctx := context.Background()

	client, err := makc.Open(
		makc.WithKeyboardInjection(makc.KeyboardInjectionSendInput),
		makc.WithMouseInjection(makc.MouseInjectionSendInput),
	)
	if err != nil {
		t.Skipf("makc backend unavailable: %v", err)
	}
	defer client.Close()

	// Restore the cursor at the end.
	start, err := client.Mouse.Position(ctx)
	if err != nil {
		t.Fatalf("position: %v", err)
	}
	t.Cleanup(func() { _ = client.Mouse.MoveTo(ctx, start.X, start.Y) })

	// --- Keyboard ---
	kc, ok := KeyCodesFromName("lshift")
	if !ok {
		t.Fatal("KeyCodesFromName(lshift) failed")
	}
	if err := MakcInputPress(client, kc, ctx); err != nil {
		t.Fatalf("keyboard press: %v", err)
	}
	if down, err := client.Keyboard.Down(ctx, kc.Makc); err != nil || !down {
		t.Fatalf("keyboard not down after press: down=%v err=%v", down, err)
	}
	if err := MakcInputRelease(client, kc, ctx); err != nil {
		t.Fatalf("keyboard release: %v", err)
	}
	if down, err := client.Keyboard.Down(ctx, kc.Makc); err != nil || down {
		t.Fatalf("keyboard still down after release: down=%v err=%v", down, err)
	}
	t.Log("keyboard press/release OK")

	// --- Mouse relative move ---
	before, _ := client.Mouse.Position(ctx)
	if err := MakcInputMouseMove(client, 50, 0, ctx); err != nil {
		t.Fatalf("mouse move: %v", err)
	}
	time.Sleep(20 * time.Millisecond)
	after, _ := client.Mouse.Position(ctx)
	if after.X == before.X && after.Y == before.Y {
		t.Fatalf("cursor did not move: %+v -> %+v", before, after)
	}
	t.Logf("mouse relative move OK (%+v -> %+v)", before, after)

	// --- Mouse absolute move to screen center ---
	size, err := client.Mouse.ScreenSize(ctx)
	if err != nil {
		t.Fatalf("screen size: %v", err)
	}
	if err := MakcInputMouseMoveTo(client, 5000, 5000, ctx); err != nil {
		t.Fatalf("mouse moveTo: %v", err)
	}
	time.Sleep(20 * time.Millisecond)
	pos, _ := client.Mouse.Position(ctx)
	cx, cy := size.X/2, size.Y/2
	if absInt(pos.X-cx) > 200 || absInt(pos.Y-cy) > 200 {
		t.Fatalf("cursor not near center: pos=%+v center=%d,%d", pos, cx, cy)
	}
	t.Logf("mouse absolute move OK (pos=%+v)", pos)

	// --- Mouse button down/up ---
	if err := MakcInputMouseButtonDown(client, MouseButtonLeft, ctx); err != nil {
		t.Fatalf("mouse button down: %v", err)
	}
	if down, err := client.Mouse.Down(ctx, makc.ButtonLeft); err != nil || !down {
		t.Fatalf("mouse button not down after press: down=%v err=%v", down, err)
	}
	if err := MakcInputMouseButtonUp(client, MouseButtonLeft, ctx); err != nil {
		t.Fatalf("mouse button up: %v", err)
	}
	if down, err := client.Mouse.Down(ctx, makc.ButtonLeft); err != nil || down {
		t.Fatalf("mouse button still down after release: down=%v err=%v", down, err)
	}
	t.Log("mouse button down/up OK")

	// --- Wheel ---
	if err := MakcInputMouseWheel(client, 1, ctx); err != nil {
		t.Fatalf("mouse wheel: %v", err)
	}
	if err := MakcInputMouseHWheel(client, 1, ctx); err != nil {
		t.Fatalf("mouse hwheel: %v", err)
	}
	t.Log("mouse wheel OK")

	t.Log("makc SendInput integration OK")
}
