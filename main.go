package main

import (
	"context"
	"log"
	"time"

	"github.com/rpdg/winput"
)

func main() {
	time.Sleep(10 * time.Second)
	log.Println("Hello World")

	// makc
	makcInputClient, err := MakcInputInit()
	if err != nil {
		log.Printf("MakcInputInit() failed: %v", err)
	}
	defer MakcInputClose(makcInputClient)
	// winput Message
	targetWindow, err := WinputInitWithWindow(false, -1)
	if err != nil {
		log.Printf("WinputInitWithWindow() failed: %v", err)
	}
	// winput Interception
	err = WinputWithInterceptionInit()
	if err != nil {
		log.Printf("WinputWithInterceptionInit() failed: %v", err)
	} // win input
	// fakerInput
	fakerInputClient, err := FakerInputInit()
	if err != nil {
		log.Printf("FakerInputInit() failed: %v", err)
	}
	defer func(fakerInputClient *FakerInputDevice) {
		err := fakerInputClient.Close()
		if err != nil {
			log.Printf("FakerInputClient.Close() failed: %v", err)
		}
	}(fakerInputClient)

	// keycode
	codeA, ok := KeyCodesFromRune('a')
	if !ok {
		log.Printf("KeyCodesFromRune('a') failed")
	}
	codeB, ok := KeyCodesFromRune('b')
	if !ok {
		log.Printf("KeyCodesFromRune('b') failed")
	}
	codeC, ok := KeyCodesFromRune('c')
	if !ok {
		log.Printf("KeyCodesFromRune('c') failed")
	}
	codeD, ok := KeyCodesFromRune('d')
	if !ok {
		log.Printf("KeyCodesFromRune('d') failed")
	}

	// run
	ctx := context.Background()

	if makcInputClient != nil {
		if err := makcInputClient.Keyboard.Tap(ctx, codeA.Makc); err != nil {
			log.Printf("MakcInputClient.Keyboard.Tap() failed: %v", err)
		}
	} else {
		log.Println("MakcInputClient is nil, skip")
	}

	if err := winput.Press(codeB.Winput); err != nil {
		log.Printf("winput.Press() failed: %v", err)
	}

	if targetWindow != nil {
		if err := targetWindow.Press(codeC.Winput); err != nil {
			log.Printf("targetWindow.Press() failed: %v", err)
		}
	} else {
		log.Println("targetWindow is nil, skip")
	}

	if fakerInputClient != nil {
		if err := fakerInputClient.Tap(codeD.FakerInput, codeD.Modifiers); err != nil {
			log.Printf("FakerInputClient.Tap() failed: %v", err)
		}
	} else {
		log.Println("FakerInputClient is nil, skip")
	}

}
