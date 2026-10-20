//go:build integration

package engine

import (
	"GoKeyMux/internal/config"
	"GoKeyMux/internal/input"
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

	prev := config.GlobalConfig.EnabledDriveName
	config.GlobalConfig.EnabledDriveName = DriveMakc
	t.Cleanup(func() { config.GlobalConfig.EnabledDriveName = prev })

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
	screenW, screenH := input.MakcScreenSize()
	if screenW <= 0 || screenH <= 0 {
		t.Fatalf("screen size not cached: %dx%d", screenW, screenH)
	}
	t.Logf("cached screen size: %dx%d", screenW, screenH)

	// Keyboard must still dispatch through the engine.
	kc, ok := input.KeyCodesFromName("shift")
	if !ok {
		t.Fatal("input.KeyCodesFromName(shift) failed")
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
	kc, ok := input.KeyCodesFromName("lshift")
	if !ok {
		t.Fatal("input.KeyCodesFromName(lshift) failed")
	}
	if err := input.MakcInputPress(client, kc, ctx); err != nil {
		t.Fatalf("keyboard press: %v", err)
	}
	if down, err := client.Keyboard.Down(ctx, kc.Makc); err != nil || !down {
		t.Fatalf("keyboard not down after press: down=%v err=%v", down, err)
	}
	if err := input.MakcInputRelease(client, kc, ctx); err != nil {
		t.Fatalf("keyboard release: %v", err)
	}
	if down, err := client.Keyboard.Down(ctx, kc.Makc); err != nil || down {
		t.Fatalf("keyboard still down after release: down=%v err=%v", down, err)
	}
	t.Log("keyboard press/release OK")

	// --- Mouse relative move ---
	before, _ := client.Mouse.Position(ctx)
	if err := input.MakcInputMouseMove(client, 50, 0, ctx); err != nil {
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
	if err := input.MakcInputMouseMoveTo(client, 5000, 5000, ctx); err != nil {
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
	if err := input.MakcInputMouseButtonDown(client, MouseButtonLeft, ctx); err != nil {
		t.Fatalf("mouse button down: %v", err)
	}
	if down, err := client.Mouse.Down(ctx, makc.ButtonLeft); err != nil || !down {
		t.Fatalf("mouse button not down after press: down=%v err=%v", down, err)
	}
	if err := input.MakcInputMouseButtonUp(client, MouseButtonLeft, ctx); err != nil {
		t.Fatalf("mouse button up: %v", err)
	}
	if down, err := client.Mouse.Down(ctx, makc.ButtonLeft); err != nil || down {
		t.Fatalf("mouse button still down after release: down=%v err=%v", down, err)
	}
	t.Log("mouse button down/up OK")

	// --- Wheel ---
	if err := input.MakcInputMouseWheel(client, 1, ctx); err != nil {
		t.Fatalf("mouse wheel: %v", err)
	}
	if err := input.MakcInputMouseHWheel(client, 1, ctx); err != nil {
		t.Fatalf("mouse hwheel: %v", err)
	}
	t.Log("mouse wheel OK")

	t.Log("makc SendInput integration OK")
}

// TestMakcReleaseMatchesHeldKeyIntegration is the real-backend counterpart of
// TestEngineReleaseUsesHeldModifiers. It presses "shift" and releases "lshift":
// both are the same physical modifier (same modifier flag) but carry different
// makc.Key values, so an exact KeyCodes match would leave the press tracked.
// The test asserts the engine's held state is cleared and the key is physically
// released. It uses the SendInput backend because GetAsyncKeyState can observe
// it, unlike the default InjectKeyboardInput backend.
func TestMakcReleaseMatchesHeldKeyIntegration(t *testing.T) {
	ctx := context.Background()

	client, err := makc.Open(makc.WithKeyboardInjection(makc.KeyboardInjectionSendInput))
	if err != nil {
		t.Skipf("makc SendInput backend unavailable: %v", err)
	}

	prevQueue := config.GlobalConfig.DispatchQueueSize
	config.GlobalConfig.DispatchQueueSize = 8
	t.Cleanup(func() { config.GlobalConfig.DispatchQueueSize = prevQueue })

	// Build the engine around the explicit SendInput client instead of calling
	// StartEngine, which would open its own auto-selected client.
	e := NewEngine()
	e.driveName = DriveMakc
	e.driveClient = client
	e.wg.Add(2)
	go e.run()
	go e.runMouse()
	t.Cleanup(func() { _ = e.CloseEngine() })

	pressed, ok := input.KeyCodesFromName("shift")
	if !ok {
		t.Fatal("input.KeyCodesFromName(shift) failed")
	}
	released, ok := input.KeyCodesFromName("lshift")
	if !ok {
		t.Fatal("input.KeyCodesFromName(lshift) failed")
	}
	t.Logf("keyboard backend=%s", client.Keyboard.InjectionBackend())
	t.Logf("pressed=%+v", pressed)
	t.Logf("released=%+v", released)

	if err := e.EnginePress(ctx, pressed); err != nil {
		t.Fatalf("press: %v", err)
	}
	if down, err := client.Keyboard.Down(ctx, pressed.Makc); err != nil || !down {
		t.Fatalf("key not down after press: down=%v err=%v", down, err)
	}
	if len(e.heldKeys) != 1 {
		t.Fatalf("held after press = %v, want exactly one key", e.heldKeys)
	}
	t.Logf("held after press: %v", e.heldKeys)

	if err := e.EngineRelease(ctx, released); err != nil {
		t.Fatalf("release: %v", err)
	}
	t.Logf("held after release: %v", e.heldKeys)
	if len(e.heldKeys) != 0 {
		t.Fatalf("held after release = %v, want empty", e.heldKeys)
	}
	if down, err := client.Keyboard.Down(ctx, pressed.Makc); err != nil || down {
		t.Fatalf("key still down after release: down=%v err=%v", down, err)
	}
	t.Logf("held before CloseEngine: %v", e.heldKeys)
	if err := e.CloseEngine(); err != nil {
		t.Fatalf("CloseEngine: %v", err)
	}
	t.Log("held state cleared, key physically released, and shutdown had nothing to release")
}
