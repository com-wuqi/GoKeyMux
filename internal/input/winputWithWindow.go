package input

import (
	"errors"
	"fmt"
	"log/slog"

	"GoKeyMux/internal/config"

	"github.com/rpdg/winput"
)

func WinputInitWithWindow() (*winput.Window, error) {
	slog.Debug("WinputInitWithWindow")
	windows, err := winput.FindByProcessName(config.GlobalConfig.WinputWindowProcessName)
	if err != nil {
		slog.Debug("Find By Process Name Error")
		return nil, err
	}
	slog.Debug("debug: length of window", "len", len(windows))
	var target *winput.Window
	target = nil
	if config.GlobalConfig.WinputWindowUseStaticIndex {
		if config.GlobalConfig.WinputWindowIndex < 0 || config.GlobalConfig.WinputWindowIndex >= len(windows) {
			return nil, errors.New("index out of range")
		}
		target = windows[config.GlobalConfig.WinputWindowIndex]
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

func WinputWithWindowPress(target *winput.Window, key KeyCodes) error {
	if key.Winput == 0 {
		return fmt.Errorf("key not supported by winput backend")
	}
	return target.KeyDown(key.Winput)
}

func WinputWithWindowRelease(target *winput.Window, key KeyCodes) error {
	if key.Winput == 0 {
		return fmt.Errorf("key not supported by winput backend")
	}
	return target.KeyUp(key.Winput)
}
