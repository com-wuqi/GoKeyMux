package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/aiwaki/makc"
	"github.com/rpdg/winput"
)

// defaultDispatchQueueSize is used when the configured dispatchQueueSize is
// not positive (e.g. an older config.json that predates the field).
const defaultDispatchQueueSize = 1024

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

// MouseButton identifies a logical mouse button, independent of any backend
// encoding. Each backend maps it to its own representation in applyMouse.
type MouseButton uint8

const (
	MouseButtonLeft MouseButton = iota + 1
	MouseButtonRight
	MouseButtonMiddle
	MouseButtonX1
	MouseButtonX2
)

// mouseEventKind selects which payload a mouseEvent carries.
type mouseEventKind uint8

const (
	mouseEventMoveRel mouseEventKind = iota + 1
	mouseEventMoveAbs
	mouseEventButton
	mouseEventWheel
	mouseEventHWheel
)

// mouseEvent is one mouse injection operation in a backend-neutral form.
type mouseEvent struct {
	kind mouseEventKind

	// dx/dy is the relative movement delta in pixels for mouseEventMoveRel.
	dx, dy int32

	// x/y is the absolute position for mouseEventMoveAbs, normalized to
	// 0..10000 (matching the gRPC MouseEvent ABS_MOVE convention). Each backend
	// scales it to its own absolute range (e.g. FakerInput's 0..32767).
	x, y int32

	// button and isPressed describe mouseEventButton.
	button    MouseButton
	isPressed bool

	// delta is the wheel detent count for mouseEventWheel / mouseEventHWheel.
	delta int32
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
}

func NewEngine() *Engine {
	size := GlobalConfig.DispatchQueueSize
	if size <= 0 {
		size = defaultDispatchQueueSize
	}
	return &Engine{
		driveClient: nil,
		ops:         make(chan keyOp, size),
		mouseOps:    make(chan mouseOp, size),
		done:        make(chan struct{}),
	}
}

func (e *Engine) StartEngine() error {
	switch GlobalConfig.EnabledDriveName {
	case DriveMakc:
		{
			client, err := MakcInputInit()
			if err != nil {
				return err
			}
			e.driveClient = client
		}
	case DriveFakerInput:
		{
			client, err := FakerInputInit()
			if err != nil {
				return err
			}
			e.driveClient = client
		}
	case DriveWinputWithWindow:
		{
			window, err := WinputInitWithWindow()
			if err != nil {
				return err
			}
			slog.Warn("使用winput可能会导致键鼠输入串行化")
			e.driveClient = window
		}
	case DriveWinputWithInterception:
		{
			if err := WinputWithInterceptionInit(); err != nil {
				return err
			}
			slog.Warn("使用winput可能会导致键鼠输入串行化")
			e.driveClient = nil
		}
	case DriveNoop:
		{
			e.driveClient = NewNoopDevice(time.Duration(GlobalConfig.NoopLatencyMicros) * time.Microsecond)
			slog.Debug("debug driver ready")
		}
	default:
		return fmt.Errorf("unsupported drive %s", GlobalConfig.EnabledDriveName)
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

func (e *Engine) CloseEngine() error {
	var err error
	switch GlobalConfig.EnabledDriveName {
	case DriveMakc:
		{
			if client, ok := e.driveClient.(*makc.Client); ok {
				err = client.Close()
			} else {
				err = fmt.Errorf("drive is not ‘*make.Client’")
			}
		}
	case DriveFakerInput:
		{
			if client, ok := e.driveClient.(*FakerInputDevice); ok {
				err = client.Close()
			} else {
				err = fmt.Errorf("drive is not '*FakerInputDevice'")
			}
		}
	case DriveWinputWithWindow, DriveWinputWithInterception:
		{
			err = nil
		}
	case DriveNoop:
		{
			err = nil
		}
	default:
		err = fmt.Errorf("unsupported drive %s", GlobalConfig.EnabledDriveName)
	}

	close(e.done)
	e.wg.Wait()
	return err
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
			return
		}
	}
}

// drain flushes any requests still queued at shutdown so no release event is
// dropped (which could otherwise leave a key stuck in the down state).
func (e *Engine) drain() {
	for {
		select {
		case op := <-e.ops:
			op.resp <- e.apply(op.ctx, op.keys, op.isPressed)
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
			return
		}
	}
}

// drainMouse flushes any mouse requests still queued at shutdown.
func (e *Engine) drainMouse() {
	for {
		select {
		case op := <-e.mouseOps:
			op.resp <- e.applyMouse(op.ctx, op.ev)
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
	return e.submitMouse(ctx, mouseEvent{kind: mouseEventMoveRel, dx: dx, dy: dy})
}

// EngineMouseMoveTo injects an absolute mouse movement to a position
// normalized to 0..10000 (matching the gRPC ABS_MOVE convention).
func (e *Engine) EngineMouseMoveTo(ctx context.Context, x, y int32) error {
	return e.submitMouse(ctx, mouseEvent{kind: mouseEventMoveAbs, x: x, y: y})
}

// EngineMouseButtonDown presses a mouse button and keeps it held.
func (e *Engine) EngineMouseButtonDown(ctx context.Context, button MouseButton) error {
	return e.submitMouse(ctx, mouseEvent{kind: mouseEventButton, button: button, isPressed: true})
}

// EngineMouseButtonUp releases a mouse button.
func (e *Engine) EngineMouseButtonUp(ctx context.Context, button MouseButton) error {
	return e.submitMouse(ctx, mouseEvent{kind: mouseEventButton, button: button, isPressed: false})
}

// EngineMouseWheel injects vertical wheel movement in detents.
func (e *Engine) EngineMouseWheel(ctx context.Context, delta int32) error {
	return e.submitMouse(ctx, mouseEvent{kind: mouseEventWheel, delta: delta})
}

// EngineMouseHWheel injects horizontal wheel movement in detents.
func (e *Engine) EngineMouseHWheel(ctx context.Context, delta int32) error {
	return e.submitMouse(ctx, mouseEvent{kind: mouseEventHWheel, delta: delta})
}

// submitMouse enqueues a mouse request and waits for the mouse worker to
// process it. It mirrors submit but uses the dedicated mouse channel.
func (e *Engine) submitMouse(ctx context.Context, ev mouseEvent) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	op := mouseOp{ctx: ctx, ev: ev, resp: make(chan error, 1)}
	select {
	case e.mouseOps <- op:
	case <-ctx.Done():
		return ctx.Err()
	case <-e.done:
		return errors.New("ctx done")
	}
	select {
	case err := <-op.resp:
		return err
	case <-ctx.Done():
		return ctx.Err()
	case <-e.done:
		return errors.New("ctx done")
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
	select {
	case e.ops <- op:
	case <-ctx.Done():
		return ctx.Err()
	case <-e.done:
		return errors.New("ctx done")
	}
	select {
	case err := <-op.resp:
		return err
	case <-ctx.Done():
		return ctx.Err()
	case <-e.done:
		return errors.New("ctx done")
	}
}

// apply performs the actual key press or release on the configured backend.
// It is only ever invoked from the worker goroutine, so it may safely assume
// exclusive access to the backend.
func (e *Engine) apply(ctx context.Context, keys []KeyCodes, isPressed bool) error {
	switch GlobalConfig.EnabledDriveName {
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
				var err error
				if isPressed {
					err = MakcInputPress(client, key, ctx)
				} else {
					err = MakcInputRelease(client, key, ctx)
				}
				if err != nil {
					return err
				}
			}
			return nil
		}
	case DriveFakerInput:
		{
			client, ok := e.driveClient.(*FakerInputDevice)
			if !ok {
				return fmt.Errorf("drive is not '*FakerInputDevice'")
			}
			for _, key := range keys {
				if err := ctx.Err(); err != nil {
					return err
				}
				var err error
				if isPressed {
					err = client.KeyDown(key.FakerInput, key.Modifiers)
				} else {
					err = client.KeyUp(key.FakerInput, key.Modifiers)
				}
				if err != nil {
					return err
				}
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
				var err error
				if isPressed {
					err = WinputWithWindowPress(target, key)
				} else {
					err = WinputWithWindowRelease(target, key)
				}
				if err != nil {
					return err
				}
			}
			return nil
		}
	case DriveWinputWithInterception:
		{
			for _, key := range keys {
				if err := ctx.Err(); err != nil {
					return err
				}
				var err error
				if isPressed {
					err = WinputWithInterceptionPress(key)
				} else {
					err = WinputWithInterceptionRelease(key)
				}
				if err != nil {
					return err
				}
			}
			return nil
		}
	case DriveNoop:
		{
			client, ok := e.driveClient.(*NoopDevice)
			if !ok {
				return fmt.Errorf("drive is not '*NoopDevice'")
			}
			for _, key := range keys {
				if err := ctx.Err(); err != nil {
					return err
				}
				var err error
				if isPressed {
					err = client.Press(ctx, key)
				} else {
					err = client.Release(ctx, key)
				}
				if err != nil {
					return err
				}
			}
			return nil
		}
	default:
		return fmt.Errorf("unsupported drive %s", GlobalConfig.EnabledDriveName)
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
	switch GlobalConfig.EnabledDriveName {
	case DriveFakerInput:
		{
			client, ok := e.driveClient.(*FakerInputDevice)
			if !ok {
				return fmt.Errorf("drive is not '*FakerInputDevice'")
			}
			switch ev.kind {
			case mouseEventMoveRel:
				return client.MouseMove(FakerInputRelDelta(ev.dx), FakerInputRelDelta(ev.dy))
			case mouseEventMoveAbs:
				return client.MouseMoveTo(FakerInputAbsCoordinate(ev.x), FakerInputAbsCoordinate(ev.y))
			case mouseEventButton:
				button, err := FakerInputMouseButton(ev.button)
				if err != nil {
					return err
				}
				if ev.isPressed {
					return client.MouseButtonDown(button)
				}
				return client.MouseButtonUp(button)
			case mouseEventWheel:
				return client.MouseWheel(int8(ev.delta))
			case mouseEventHWheel:
				return client.MouseHWheel(int8(ev.delta))
			default:
				return fmt.Errorf("unknown mouse event %d", ev.kind)
			}
		}
	case DriveNoop:
		{
			client, ok := e.driveClient.(*NoopDevice)
			if !ok {
				return fmt.Errorf("drive is not '*NoopDevice'")
			}
			return client.Mouse(ctx, ev)
		}
	case DriveMakc:
		{
			client, ok := e.driveClient.(*makc.Client)
			if !ok {
				return fmt.Errorf("drive is not '*makc.Client'")
			}
			switch ev.kind {
			case mouseEventMoveRel:
				return MakcInputMouseMove(client, ev.dx, ev.dy, ctx)
			case mouseEventMoveAbs:
				return MakcInputMouseMoveTo(client, ev.x, ev.y, ctx)
			case mouseEventButton:
				if ev.isPressed {
					return MakcInputMouseButtonDown(client, ev.button, ctx)
				}
				return MakcInputMouseButtonUp(client, ev.button, ctx)
			case mouseEventWheel:
				return MakcInputMouseWheel(client, ev.delta, ctx)
			case mouseEventHWheel:
				return MakcInputMouseHWheel(client, ev.delta, ctx)
			default:
				return fmt.Errorf("unknown mouse event %d", ev.kind)
			}
		}
	case DriveWinputWithWindow, DriveWinputWithInterception:
		return fmt.Errorf("mouse not implemented for drive %s", GlobalConfig.EnabledDriveName)
	default:
		return fmt.Errorf("unsupported drive %s", GlobalConfig.EnabledDriveName)
	}
}
