package main

import (
	"context"
	"log"
	"time"

	"github.com/aiwaki/makc"
	"github.com/rpdg/winput"
)

func main() {
	time.Sleep(10 * time.Second)
	log.Println("Hello World")
	makcInputClient, err := MakcInputInit()
	if err != nil {
		log.Printf("MakcInputInit() failed: %v", err)
	}
	defer MakcInputClose(makcInputClient)

	_, err = WinInputInitWithWindow(false, -1)
	if err != nil {
		log.Printf("WinInputInitWithWindow() failed: %v", err)
	}

	err = WinInputWithInterceptionInit()
	if err != nil {
		log.Printf("WinInputWithInterceptionInit() failed: %v", err)
	} // win input

	ctx := context.Background()
	for {
		time.Sleep(300 * time.Millisecond)
		err := makcInputClient.Keyboard.Tap(ctx, makc.KeyZ)
		if err != nil {
			log.Printf("MakcInputClient.Keyboard.Tap() failed: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
		err = winput.Press(winput.KeyX)
		if err != nil {
			log.Printf("winput.Press() failed: %v", err)
		}
	}

}
