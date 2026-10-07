package main

import (
	"context"
	"errors"
	"fmt"
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

type Engine struct {
	driveClient any

	// ops is the single serialization point for key injection. Every press or
	// release is submitted to this buffered channel and processed by exactly
	// one worker goroutine, so the underlying backends are only ever touched
	// by a single goroutine at a time. Blocked senders are queued FIFO by the
	// runtime, so ordering is preserved even when the buffer is full.
	ops  chan keyOp
	done chan struct{}
	wg   sync.WaitGroup
}

func NewEngine() *Engine {
	size := GlobalConfig.DispatchQueueSize
	if size <= 0 {
		size = defaultDispatchQueueSize
	}
	return &Engine{
		driveClient: nil,
		ops:         make(chan keyOp, size),
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
			e.driveClient = window
		}
	case DriveWinputWithInterception:
		{
			if err := WinputWithInterceptionInit(); err != nil {
				return err
			}
			e.driveClient = nil
		}
	case DriveNoop:
		{
			e.driveClient = NewNoopDevice(time.Duration(GlobalConfig.NoopLatencyMicros) * time.Microsecond)
		}
	default:
		return fmt.Errorf("unsupported drive %s", GlobalConfig.EnabledDriveName)
	}

	// Start the worker after the backend is initialized so the goroutine sees
	// the fully populated driveClient (the go statement establishes a
	// happens-before edge).
	e.wg.Add(1)
	go e.run()
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

func (e *Engine) EnginePress(ctx context.Context, keys ...KeyCodes) error {
	return e.submit(ctx, true, keys)
}

func (e *Engine) EngineRelease(ctx context.Context, keys ...KeyCodes) error {
	return e.submit(ctx, false, keys)
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
