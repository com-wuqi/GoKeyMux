package main

import (
	"fmt"

	"github.com/aiwaki/makc"
)

type Engine struct {
	driveClient any
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
			e.driveClient = nil
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
	default:
		return fmt.Errorf("unsupported drive %s", GlobalConfig.EnabledDriveName)
	}
}

func (e *Engine) EnginePress() error {
	// TODO
	return fmt.Errorf("unavailable")
}

func (e *Engine) EngineRelease() error {
	// TODO
	return fmt.Errorf("unavailable")
}
