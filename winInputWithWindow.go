package main

import (
	"errors"
	"log"

	"github.com/rpdg/winput"
)

func WinInputInitWithWindow(useStaticIndex bool, index int) (*winput.Window, error) {
	log.Println("WinInputInitWithWindow")
	windows, err := winput.FindByProcessName("notepad.exe")
	if err != nil {
		log.Println("Find By Process Name Error")
		return nil, err
	}
	log.Println("debug: length of window:", len(windows))
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
		log.Println("WinInputInitWithWindow: window not found")
		return nil, errors.New("WinInputInitWithWindow: window not found")
	}
	err = target.Press(winput.KeyB)
	if err != nil {
		log.Println("WinInputInitWithWindow Press Error")
		return nil, err
	}
	return target, err
}
