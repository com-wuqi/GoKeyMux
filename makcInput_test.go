package main

import (
	"testing"

	"github.com/aiwaki/makc"
)

func TestMakcMouseButton(t *testing.T) {
	cases := []struct {
		in   MouseButton
		want makc.MouseButton
	}{
		{MouseButtonLeft, makc.ButtonLeft},
		{MouseButtonRight, makc.ButtonRight},
		{MouseButtonMiddle, makc.ButtonMiddle},
		{MouseButtonX1, makc.ButtonX1},
		{MouseButtonX2, makc.ButtonX2},
	}
	for _, c := range cases {
		got, err := MakcMouseButton(c.in)
		if err != nil {
			t.Fatalf("MakcMouseButton(%d): %v", c.in, err)
		}
		if got != c.want {
			t.Errorf("MakcMouseButton(%d) = %d, want %d", c.in, got, c.want)
		}
	}

	if _, err := MakcMouseButton(MouseButton(99)); err == nil {
		t.Error("expected error for unknown mouse button")
	}
}

func TestMakcAbsCoordinate(t *testing.T) {
	const size = 1920
	cases := []struct {
		in   int32
		want int
	}{
		{0, 0},
		{10000, size},
		{5000, size / 2},
		{-1, 0},
		{10001, size},
	}
	for _, c := range cases {
		if got := MakcAbsCoordinate(c.in, size); got != c.want {
			t.Errorf("MakcAbsCoordinate(%d, %d) = %d, want %d", c.in, size, got, c.want)
		}
	}
}
