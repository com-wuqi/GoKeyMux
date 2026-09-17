package main

import (
	"log/slog"

	"github.com/aiwaki/makc"
)

func MakcInputInit() (*makc.Client, error) {
	slog.Debug("FakerInputInit")
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
		slog.Warn("FakerInputClose failed")
	}
}
