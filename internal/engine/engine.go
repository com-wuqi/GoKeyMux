package engine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"GoKeyMux/internal/config"
	"GoKeyMux/internal/input"

	"github.com/aiwaki/makc"
	"github.com/rpdg/winput"
)

// defaultDispatchQueueSize is used when the configured dispatchQueueSize is
// not positive (e.g. an older config.json that predates the field).
const defaultDispatchQueueSize = 1024

// ErrEngineClosed is returned when a request is submitted after CloseEngine has
// started shutting the engine down.
var ErrEngineClosed = errors.New("engine closed")

// keyOp is a single key injection request submitted to the engine worker.
type keyOp struct {
	ctx       context.Context
	isPressed bool
	keys      []KeyCodes
	// resp carries the dispatch error back to the submitter. It is buffered
	// with a single slot so the worker never blocks if the submitter has
	// already been canceled or the engine shut down.
	resp chan error
}

// mouseOp is a single mouse injection request submitted to the engine mouse
// worker. It is the mouse counterpart of keyOp.
type mouseOp struct {
	ctx  context.Context
	ev   mouseEvent
	resp chan error
}

type Engine struct {
	driveClient any
	// driveName is captured at StartEngine time. Using it instead of reading
	// config.GlobalConfig at dispatch/close time keeps the whole lifecycle bound to a
	// single backend even if the global config is mutated later.
	driveName DriveName

	// ops is the serialization point for key injection; mouseOps is the
	// independent serialization point for mouse injection. Keyboard and mouse
	// are processed by separate worker goroutines so they can be dispatched
	// simultaneously without blocking each other. Blocked senders are queued
	// FIFO by the runtime, so ordering within each channel is preserved even
	// when the buffer is full.
	ops      chan keyOp
	mouseOps chan mouseOp
	done     chan struct{}
	wg       sync.WaitGroup

	// closing is set before done is closed. Submitters check it first, so a
	// request arriving after shutdown starts fails fast instead of racing the
	// queue drain. Ordering is handled by the shutdown sequence in main (the
	// engine stops accepting work before gRPC is stopped), which keeps the
	// queue quiescent without a hot-path lock.
	closing atomic.Bool

	closeOnce sync.Once
	closeErr  error

	// Held input state. The key fields are only read/written by the keyboard
	// worker; the button fields only by the mouse worker. They are used to
	// release everything still down when the engine shuts down. Keeping state
	// here (rather than querying the backend) is essential: backend state
	// queries such as makc's KeyState reflect physical keys too, so releasing
	// based on them could release keys the user is holding.
	heldKeys      []KeyCodes
	heldKeySet    map[KeyCodes]struct{}
	heldButtons   []MouseButton
	heldButtonSet map[MouseButton]struct{}
}

func NewEngine() *Engine {
	size := config.GlobalConfig.DispatchQueueSize
	if size <= 0 {
		size = defaultDispatchQueueSize
	}
	return &Engine{
		driveClient:   nil,
		ops:           make(chan keyOp, size),
		mouseOps:      make(chan mouseOp, size),
		done:          make(chan struct{}),
		heldKeySet:    make(map[KeyCodes]struct{}),
		heldButtonSet: make(map[MouseButton]struct{}),
	}
}

func (e *Engine) StartEngine() error {
	e.driveName = config.GlobalConfig.EnabledDriveName
	switch e.driveName {
	case DriveMakc:
		{
			client, err := input.MakcInputInit()
			if err != nil {
				return err
			}
			e.driveClient = client
		}
	case DriveFakerInput:
		{
			client, err := input.FakerInputInit()
			if err != nil {
				return err
			}
			e.driveClient = client
		}
	case DriveWinputWithWindow:
		{
			window, err := input.WinputInitWithWindow()
			if err != nil {
				return err
			}
			slog.Warn("使用winput可能会导致键鼠输入串行化")
			e.driveClient = window
		}
	case DriveWinputWithInterception:
		{
			if err := input.WinputWithInterceptionInit(); err != nil {
				return err
			}
			slog.Warn("使用winput可能会导致键鼠输入串行化")
			e.driveClient = nil
		}
	case DriveNoop:
		{
			e.driveClient = input.NewNoopDevice(time.Duration(config.GlobalConfig.NoopLatencyMicros) * time.Microsecond)
			slog.Debug("debug driver ready")
		}
	default:
		return fmt.Errorf("unsupported drive %s", config.GlobalConfig.EnabledDriveName)
	}

	// Start the workers after the backend is initialized so the goroutines see
	// the fully populated driveClient (the go statement establishes a
	// happens-before edge). Keyboard and mouse run on separate workers so they
	// can inject simultaneously.
	e.wg.Add(2)
	go e.run()
	go e.runMouse()
	return nil
}

// CloseEngine stops the workers and closes the backend. Before the backend is
// closed, any requests still queued are drained and every key/mouse button that
// is still held is released, so a graceful shutdown never leaves a key stuck
// down. It is safe to call CloseEngine more than once and from multiple
// goroutines.
func (e *Engine) CloseEngine() error {
	e.closeOnce.Do(func() {
		// Reject new requests before closing done. A submit that read the old
		// value just before this store may still enqueue one op, but that op is
		// never consumed: the caller gets ErrEngineClosed and the held-input
		// cleanup below still guarantees no key is left down.
		e.closing.Store(true)
		close(e.done)

		// The workers drain pending ops and release held input before they
		// return, so waiting here ensures the backend is no longer in use.
		e.wg.Wait()

		switch e.driveName {
		case DriveMakc:
			{
				if client, ok := e.driveClient.(*makc.Client); ok {
					e.closeErr = client.Close()
				} else if e.driveClient != nil {
					e.closeErr = fmt.Errorf("drive is not '*makc.Client'")
				}
			}
		case DriveFakerInput:
			{
				if client, ok := e.driveClient.(*input.FakerInputDevice); ok {
					e.closeErr = client.Close()
				} else if e.driveClient != nil {
					e.closeErr = fmt.Errorf("drive is not '*input.FakerInputDevice'")
				}
			}
		case DriveWinputWithWindow, DriveWinputWithInterception:
			{
				e.closeErr = nil
			}
		case DriveNoop:
			{
				if client, ok := e.driveClient.(*input.NoopDevice); ok {
					e.closeErr = client.Close()
				} else if e.driveClient != nil {
					e.closeErr = fmt.Errorf("drive is not '*input.NoopDevice'")
				}
			}
		default:
			if e.driveName != "" {
				e.closeErr = fmt.Errorf("unsupported drive %s", e.driveName)
			}
		}
	})
	return e.closeErr
}

// run is the single worker that drains the request queue. Because only this
// goroutine ever touches the backends, no mutex is required to serialize key
// injection.
func (e *Engine) run() {
	defer e.wg.Done()
	for {
		select {
		case op := <-e.ops:
			op.resp <- e.apply(op.ctx, op.keys, op.isPressed)
		case <-e.done:
			e.drain()
			e.releaseHeldKeys()
			return
		}
	}
}

// drain flushes any requests still queued at shutdown so no release event is
// dropped (which could otherwise leave a key stuck in the down state). A
// release is always applied with a detached context: the request's context may
// already be canceled (for example because the client stream was aborted), but
// skipping the release would leave the key down.
func (e *Engine) drain() {
	for {
		select {
		case op := <-e.ops:
			ctx := op.ctx
			if !op.isPressed {
				ctx = context.Background()
			}
			op.resp <- e.apply(ctx, op.keys, op.isPressed)
		default:
			return
		}
	}
}

// runMouse is the single worker that drains the mouse request queue. It runs
// concurrently with run, so mouse and keyboard injection do not serialize
// against each other.
func (e *Engine) runMouse() {
	defer e.wg.Done()
	for {
		select {
		case op := <-e.mouseOps:
			op.resp <- e.applyMouse(op.ctx, op.ev)
		case <-e.done:
			e.drainMouse()
			e.releaseHeldButtons()
			return
		}
	}
}

// drainMouse flushes any mouse requests still queued at shutdown. Button
// releases use a detached context for the same reason as in drain.
func (e *Engine) drainMouse() {
	for {
		select {
		case op := <-e.mouseOps:
			ctx := op.ctx
			if op.ev.Kind == mouseEventButton && !op.ev.IsPressed {
				ctx = context.Background()
			}
			op.resp <- e.applyMouse(ctx, op.ev)
		default:
			return
		}
	}
}

func (e *Engine) EnginePress(ctx context.Context, keys ...KeyCodes) error {
	return e.submit(ctx, true, keys)
}

func (e *Engine) EngineRelease(ctx context.Context, keys ...KeyCodes) error {
	return e.submit(ctx, false, keys)
}

// EngineMouseMove injects a relative mouse movement by dx/dy pixels.
func (e *Engine) EngineMouseMove(ctx context.Context, dx, dy int32) error {
	return e.submitMouse(ctx, mouseEvent{Kind: mouseEventMoveRel, DX: dx, DY: dy})
}

// EngineMouseMoveTo injects an absolute mouse movement to a position
// normalized to 0..10000 (matching the gRPC ABS_MOVE convention).
func (e *Engine) EngineMouseMoveTo(ctx context.Context, x, y int32) error {
	return e.submitMouse(ctx, mouseEvent{Kind: mouseEventMoveAbs, X: x, Y: y})
}

// EngineMouseButtonDown presses a mouse button and keeps it held.
func (e *Engine) EngineMouseButtonDown(ctx context.Context, button MouseButton) error {
	return e.submitMouse(ctx, mouseEvent{Kind: mouseEventButton, Button: button, IsPressed: true})
}

// EngineMouseButtonUp releases a mouse button.
func (e *Engine) EngineMouseButtonUp(ctx context.Context, button MouseButton) error {
	return e.submitMouse(ctx, mouseEvent{Kind: mouseEventButton, Button: button, IsPressed: false})
}

// EngineMouseWheel injects vertical wheel movement in detents.
func (e *Engine) EngineMouseWheel(ctx context.Context, delta int32) error {
	return e.submitMouse(ctx, mouseEvent{Kind: mouseEventWheel, Delta: delta})
}

// EngineMouseHWheel injects horizontal wheel movement in detents.
func (e *Engine) EngineMouseHWheel(ctx context.Context, delta int32) error {
	return e.submitMouse(ctx, mouseEvent{Kind: mouseEventHWheel, Delta: delta})
}

// submitMouse enqueues a mouse request and waits for the mouse worker to
// process it. It mirrors submit but uses the dedicated mouse channel.
func (e *Engine) submitMouse(ctx context.Context, ev mouseEvent) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	op := mouseOp{ctx: ctx, ev: ev, resp: make(chan error, 1)}
	if e.closing.Load() {
		return ErrEngineClosed
	}
	select {
	case e.mouseOps <- op:
	case <-ctx.Done():
		return ctx.Err()
	case <-e.done:
		return ErrEngineClosed
	}
	select {
	case err := <-op.resp:
		return err
	case <-ctx.Done():
		return ctx.Err()
	case <-e.done:
		return ErrEngineClosed
	}
}

// submit enqueues a request and waits for the worker to process it. A buffered
// queue gives bounded burst capacity; once full, senders block FIFO. A request
// whose context is already canceled never enters the queue.
func (e *Engine) submit(ctx context.Context, isPressed bool, keys []KeyCodes) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	op := keyOp{ctx: ctx, isPressed: isPressed, keys: keys, resp: make(chan error, 1)}
	if e.closing.Load() {
		return ErrEngineClosed
	}
	select {
	case e.ops <- op:
	case <-ctx.Done():
		return ctx.Err()
	case <-e.done:
		return ErrEngineClosed
	}
	select {
	case err := <-op.resp:
		return err
	case <-ctx.Done():
		return ctx.Err()
	case <-e.done:
		return ErrEngineClosed
	}
}

// apply performs the actual key press or release on the configured backend.
// It is only ever invoked from the worker goroutine, so it may safely assume
// exclusive access to the backend.
func (e *Engine) apply(ctx context.Context, keys []KeyCodes, isPressed bool) error {
	switch e.driveName {
	case DriveMakc:
		{
			client, ok := e.driveClient.(*makc.Client)
			if !ok {
				return fmt.Errorf("drive is not '*makc.Client'")
			}
			for _, key := range keys {
				if err := ctx.Err(); err != nil {
					return err
				}
				key = e.effectiveKey(key, isPressed)
				var err error
				if isPressed {
					err = input.MakcInputPress(client, key, ctx)
				} else {
					err = input.MakcInputRelease(client, key, ctx)
				}
				if err != nil {
					return err
				}
				e.recordKeyState(key, isPressed)
			}
			return nil
		}
	case DriveFakerInput:
		{
			client, ok := e.driveClient.(*input.FakerInputDevice)
			if !ok {
				return fmt.Errorf("drive is not '*input.FakerInputDevice'")
			}
			for _, key := range keys {
				if err := ctx.Err(); err != nil {
					return err
				}
				key = e.effectiveKey(key, isPressed)
				var err error
				if isPressed {
					err = client.KeyDown(key.FakerInput, key.Modifiers)
				} else {
					err = client.KeyUp(key.FakerInput, key.Modifiers)
				}
				if err != nil {
					return err
				}
				e.recordKeyState(key, isPressed)
			}
			return nil
		}
	case DriveWinputWithWindow:
		{
			target, ok := e.driveClient.(*winput.Window)
			if !ok {
				return fmt.Errorf("drive is not '*winput.Window'")
			}
			for _, key := range keys {
				if err := ctx.Err(); err != nil {
					return err
				}
				key = e.effectiveKey(key, isPressed)
				var err error
				if isPressed {
					err = input.WinputWithWindowPress(target, key)
				} else {
					err = input.WinputWithWindowRelease(target, key)
				}
				if err != nil {
					return err
				}
				e.recordKeyState(key, isPressed)
			}
			return nil
		}
	case DriveWinputWithInterception:
		{
			for _, key := range keys {
				if err := ctx.Err(); err != nil {
					return err
				}
				key = e.effectiveKey(key, isPressed)
				var err error
				if isPressed {
					err = input.WinputWithInterceptionPress(key)
				} else {
					err = input.WinputWithInterceptionRelease(key)
				}
				if err != nil {
					return err
				}
				e.recordKeyState(key, isPressed)
			}
			return nil
		}
	case DriveNoop:
		{
			client, ok := e.driveClient.(*input.NoopDevice)
			if !ok {
				return fmt.Errorf("drive is not '*input.NoopDevice'")
			}
			for _, key := range keys {
				if err := ctx.Err(); err != nil {
					return err
				}
				key = e.effectiveKey(key, isPressed)
				var err error
				if isPressed {
					err = client.Press(ctx, key)
				} else {
					err = client.Release(ctx, key)
				}
				if err != nil {
					return err
				}
				e.recordKeyState(key, isPressed)
			}
			return nil
		}
	default:
		return fmt.Errorf("unsupported drive %s", e.driveName)
	}
}

// applyMouse performs a single mouse operation on the configured backend. It is
// only ever invoked from the mouse worker goroutine. Each driver maps the
// backend-neutral mouseEvent to its own representation; drivers that do not
// implement mouse injection yet return an explicit error.
func (e *Engine) applyMouse(ctx context.Context, ev mouseEvent) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	switch e.driveName {
	case DriveFakerInput:
		{
			client, ok := e.driveClient.(*input.FakerInputDevice)
			if !ok {
				return fmt.Errorf("drive is not '*input.FakerInputDevice'")
			}
			switch ev.Kind {
			case mouseEventMoveRel:
				return client.MouseMove(input.FakerInputRelDelta(ev.DX), input.FakerInputRelDelta(ev.DY))
			case mouseEventMoveAbs:
				return client.MouseMoveTo(input.FakerInputAbsCoordinate(ev.X), input.FakerInputAbsCoordinate(ev.Y))
			case mouseEventButton:
				button, err := input.FakerInputMouseButton(ev.Button)
				if err != nil {
					return err
				}
				if ev.IsPressed {
					if err := client.MouseButtonDown(button); err != nil {
						return err
					}
				} else {
					if err := client.MouseButtonUp(button); err != nil {
						return err
					}
				}
				e.recordMouseButton(ev.Button, ev.IsPressed)
				return nil
			case mouseEventWheel:
				return client.MouseWheel(int8(ev.Delta))
			case mouseEventHWheel:
				return client.MouseHWheel(int8(ev.Delta))
			default:
				return fmt.Errorf("unknown mouse event %d", ev.Kind)
			}
		}
	case DriveNoop:
		{
			client, ok := e.driveClient.(*input.NoopDevice)
			if !ok {
				return fmt.Errorf("drive is not '*input.NoopDevice'")
			}
			if err := client.Mouse(ctx, ev); err != nil {
				return err
			}
			if ev.Kind == mouseEventButton {
				e.recordMouseButton(ev.Button, ev.IsPressed)
			}
			return nil
		}
	case DriveMakc:
		{
			client, ok := e.driveClient.(*makc.Client)
			if !ok {
				return fmt.Errorf("drive is not '*makc.Client'")
			}
			switch ev.Kind {
			case mouseEventMoveRel:
				return input.MakcInputMouseMove(client, ev.DX, ev.DY, ctx)
			case mouseEventMoveAbs:
				return input.MakcInputMouseMoveTo(client, ev.X, ev.Y, ctx)
			case mouseEventButton:
				if ev.IsPressed {
					if err := input.MakcInputMouseButtonDown(client, ev.Button, ctx); err != nil {
						return err
					}
				} else {
					if err := input.MakcInputMouseButtonUp(client, ev.Button, ctx); err != nil {
						return err
					}
				}
				e.recordMouseButton(ev.Button, ev.IsPressed)
				return nil
			case mouseEventWheel:
				return input.MakcInputMouseWheel(client, ev.Delta, ctx)
			case mouseEventHWheel:
				return input.MakcInputMouseHWheel(client, ev.Delta, ctx)
			default:
				return fmt.Errorf("unknown mouse event %d", ev.Kind)
			}
		}
	case DriveWinputWithWindow, DriveWinputWithInterception:
		return fmt.Errorf("mouse not implemented for drive %s", e.driveName)
	default:
		return fmt.Errorf("unsupported drive %s", e.driveName)
	}
}

// effectiveKey resolves which key to send to the backend. For a press it is the
// requested key. For a release it prefers the exact held entry, then falls back
// to a held entry that identifies the same physical key but differs in
// modifiers (for example "A" pressed with Shift and "a" released without it).
// Using the held entry lets the backend release exactly the state it recorded
// and removes the matching tracking entry, so shutdown does not send a spurious
// extra release and FakerInput does not leave the modifier held.
func (e *Engine) effectiveKey(key KeyCodes, isPressed bool) KeyCodes {
	if isPressed {
		return key
	}
	if _, ok := e.heldKeySet[key]; ok {
		return key
	}
	for _, held := range e.heldKeys {
		if input.SamePhysicalKey(held, key) {
			return held
		}
	}
	return key
}

// recordKeyState updates the set of held keys after a backend injection
// succeeded. It must only be called from the keyboard worker goroutine.
func (e *Engine) recordKeyState(key KeyCodes, isPressed bool) {
	if isPressed {
		if _, ok := e.heldKeySet[key]; ok {
			return
		}
		e.heldKeySet[key] = struct{}{}
		e.heldKeys = append(e.heldKeys, key)
		return
	}
	if _, ok := e.heldKeySet[key]; !ok {
		return
	}
	delete(e.heldKeySet, key)
	for i, held := range e.heldKeys {
		if held == key {
			e.heldKeys = append(e.heldKeys[:i], e.heldKeys[i+1:]...)
			return
		}
	}
}

// recordMouseButton updates the set of held mouse buttons after a backend
// injection succeeded. It must only be called from the mouse worker goroutine.
func (e *Engine) recordMouseButton(button MouseButton, isPressed bool) {
	if isPressed {
		if _, ok := e.heldButtonSet[button]; ok {
			return
		}
		e.heldButtonSet[button] = struct{}{}
		e.heldButtons = append(e.heldButtons, button)
		return
	}
	if _, ok := e.heldButtonSet[button]; !ok {
		return
	}
	delete(e.heldButtonSet, button)
	for i, held := range e.heldButtons {
		if held == button {
			e.heldButtons = append(e.heldButtons[:i], e.heldButtons[i+1:]...)
			return
		}
	}
}

// releaseHeldKeys releases every key still held by the engine. It runs on the
// keyboard worker during shutdown, after the request queue has been drained.
// The state is snapshot and cleared first so the per-key release path cannot
// mutate the slice being iterated. Release failures are logged and do not stop
// the remaining keys from being released.
func (e *Engine) releaseHeldKeys() {
	if len(e.heldKeys) == 0 {
		return
	}
	keys := append([]KeyCodes(nil), e.heldKeys...)
	e.heldKeys = nil
	clear(e.heldKeySet)
	slog.Warn("releasing held keys on shutdown", "keys", keys)
	for _, key := range slices.Backward(keys) {
		if err := e.apply(context.Background(), []KeyCodes{key}, false); err != nil {
			slog.Warn("release held key on shutdown failed",
				"makc", key.Makc,
				"winput", key.Winput,
				"fakerInput", key.FakerInput,
				"modifiers", key.Modifiers,
				"err", err,
			)
		}
	}
}

// releaseHeldButtons releases every mouse button still held by the engine. It
// runs on the mouse worker during shutdown, after the queue has been drained.
func (e *Engine) releaseHeldButtons() {
	if len(e.heldButtons) == 0 {
		return
	}
	buttons := append([]MouseButton(nil), e.heldButtons...)
	e.heldButtons = nil
	clear(e.heldButtonSet)
	slog.Debug("releasing held mouse buttons on shutdown", "count", len(buttons))
	for _, button := range slices.Backward(buttons) {
		if err := e.applyMouse(context.Background(), mouseEvent{Kind: mouseEventButton, Button: button, IsPressed: false}); err != nil {
			slog.Warn("release held mouse button on shutdown failed", "button", button, "err", err)
		}
	}
}
