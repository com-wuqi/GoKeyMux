package main

import (
	"errors"
	"log/slog"

	"github.com/rpdg/winput"
)

func WinputInitWithWindow(useStaticIndex bool, index int) (*winput.Window, error) {
	slog.Debug("WinputInitWithWindow")
	windows, err := winput.FindByProcessName("notepad.exe")
	if err != nil {
		slog.Debug("Find By Process Name Error")
		return nil, err
	}
	slog.Debug("debug: length of window", "len", len(windows))
	var target *winput.Window
	target = nil
	if useStaticIndex {
		if index < 0 || index >= len(windows) {
			return nil, errors.New("index out of range")
		}
		target = windows[index]
	} else {
		for _, window := range windows {
			if window.IsValid() && window.IsVisible() {
				target = window
				break
			}
		}
	}
	if target == nil {
		slog.Debug("WinputInitWithWindow: window not found")
		return nil, errors.New("WinputInitWithWindow: window not found")
	}
	return target, err
}

func WinputPress(target *winput.Window, key KeyCodes) error {
	return target.KeyDown(key.Winput)
}

func WinputRelease(target *winput.Window, key KeyCodes) error {
	return target.KeyUp(key.Winput)
}
