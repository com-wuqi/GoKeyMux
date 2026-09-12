package main

import (
	"context"
	"log"

	"github.com/aiwaki/makc"
)

func MakcInputInit() (*makc.Client, error) {
	log.Println("MakcInputInit")
	client, err := makc.Open()
	if err != nil {
		return nil, nil
	}
	ctx := context.Background()
	if err := client.Keyboard.Tap(ctx, makc.KeyA); err != nil {
		log.Printf("Tap failed: %v", err)
	}
	return client, nil
}

func MakcInputClose(client *makc.Client) {
	log.Println("MakcInputClose")
	err := client.Close()
	if err != nil {
		log.Printf("MakcInputClose failed: %v", err)
	}
}
