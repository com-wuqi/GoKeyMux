package main

import (
	"log"
	"time"
)

func main() {
	time.Sleep(10 * time.Second)
	log.Println("Hello World")
	makcInputClient, err := MakcInputInit()
	if err != nil {
		log.Printf("MakcInputInit() failed: %v", err)
	}
	MakcInputClose(makcInputClient)

	_, err = WinInputInitWithWindow(false, -1)
	if err != nil {
		log.Printf("WinInputInitWithWindow() failed: %v", err)
	}

	err = WinInputWithInterceptionInit()
	if err != nil {
		log.Printf("WinInputWithInterceptionInit() failed: %v", err)
	}

}
