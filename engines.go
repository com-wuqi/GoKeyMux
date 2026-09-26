package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aiwaki/makc"
	"github.com/rpdg/winput"
)

type Engine struct {
	driveClient any

	// mu serializes access to the makc and winput backends, whose underlying
	// libraries are not safe for concurrent key injection. FakerInput
	// serializes internally (FakerInputDevice.mu) and noop is concurrency-safe.
	mu sync.Mutex
}

func NewEngine() *Engine {
	return &Engine{driveClient: nil}
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
	return nil

}

func (e *Engine) CloseEngine() error {
	switch GlobalConfig.EnabledDriveName {
	case DriveMakc:
		{
			if client, ok := e.driveClient.(*makc.Client); ok {
				return client.Close()
			}

			return fmt.Errorf("drive is not ‘*make.Client’")
		}
	case DriveFakerInput:
		{
			if client, ok := e.driveClient.(*FakerInputDevice); ok {
				return client.Close()
			}
			return fmt.Errorf("drive is not '*FakerInputDevice'")
		}
	case DriveWinputWithWindow, DriveWinputWithInterception:
		{
			return nil
		}
	case DriveNoop:
		{
			return nil
		}
	default:
		return fmt.Errorf("unsupported drive %s", GlobalConfig.EnabledDriveName)
	}
}

// needsSerialization reports whether the given backend requires serialized key
// injection. FakerInput serializes internally (FakerInputDevice.mu) and noop is
// concurrency-safe, so only makc and the winput backends are serialized here.
func needsSerialization(drive DriveName) bool {
	switch drive {
	case DriveMakc, DriveWinputWithWindow, DriveWinputWithInterception:
		return true
	}
	return false
}

func (e *Engine) EnginePress(ctx context.Context, keys ...KeyCodes) error {
	if needsSerialization(GlobalConfig.EnabledDriveName) {
		e.mu.Lock()
		defer e.mu.Unlock()
	}
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
				if err := MakcInputPress(client, key, ctx); err != nil {
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
				if err := client.KeyDown(key.FakerInput, key.Modifiers); err != nil {
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
				if err := WinputWithWindowPress(target, key); err != nil {
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
				if err := WinputWithInterceptionPress(key); err != nil {
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
				if err := client.Press(ctx, key); err != nil {
					return err
				}
			}
			return nil
		}
	default:
		return fmt.Errorf("unsupported drive %s", GlobalConfig.EnabledDriveName)
	}
}

func (e *Engine) EngineRelease(ctx context.Context, keys ...KeyCodes) error {
	if needsSerialization(GlobalConfig.EnabledDriveName) {
		e.mu.Lock()
		defer e.mu.Unlock()
	}
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
				if err := MakcInputRelease(client, key, ctx); err != nil {
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
				if err := client.KeyUp(key.FakerInput, key.Modifiers); err != nil {
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
				if err := WinputWithWindowRelease(target, key); err != nil {
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
				if err := WinputWithInterceptionRelease(key); err != nil {
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
				if err := client.Release(ctx, key); err != nil {
					return err
				}
			}
			return nil
		}
	default:
		return fmt.Errorf("unsupported drive %s", GlobalConfig.EnabledDriveName)
	}
}
