package main

import (
	"log"

	"github.com/aiwaki/makc"
)

func MakcInputInit() (*makc.Client, error) {
	log.Println("MakcInputInit")
	client, err := makc.Open()
	if err != nil {
		return nil, err
	}
	return client, nil
}

func MakcInputClose(client *makc.Client) {
	if client == nil {
		return
	}
	err := client.Close()
	if err != nil {
		log.Printf("MakcInputClose failed: %v", err)
	}
}
