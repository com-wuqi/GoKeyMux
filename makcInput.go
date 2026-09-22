package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/aiwaki/makc"
)

func MakcInputInit() (*makc.Client, error) {
	slog.Debug("MakcInputInit")
	client, err := makc.Open()
	if err != nil {
		return nil, err
	}
	return client, nil
}

func MakcInputPress(client *makc.Client, key KeyCodes, ctx context.Context) error {
	if client == nil {
		slog.Error("MakcInputPress failed: client is nil")
		return fmt.Errorf("makc client is nil")
	}
	return client.Keyboard.Press(ctx, key.Makc)
}

func MakcInputRelease(client *makc.Client, key KeyCodes, ctx context.Context) error {
	if client == nil {
		slog.Error("MakcInputRelease failed: client is nil")
		return fmt.Errorf("makc client is nil")
	}
	return client.Keyboard.Release(ctx, key.Makc)
}
