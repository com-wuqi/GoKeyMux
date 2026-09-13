package main

import (
	"errors"
	"log"

	"github.com/rpdg/winput"
)

func WinputInitWithWindow(useStaticIndex bool, index int) (*winput.Window, error) {
	log.Println("WinputInitWithWindow")
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
		log.Println("WinputInitWithWindow: window not found")
		return nil, errors.New("WinputInitWithWindow: window not found")
	}
	return target, err
}
