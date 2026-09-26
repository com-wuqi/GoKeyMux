package main

import "testing"

func TestNeedsSerialization(t *testing.T) {
	cases := []struct {
		drive DriveName
		want  bool
	}{
		{DriveMakc, true},
		{DriveWinputWithWindow, true},
		{DriveWinputWithInterception, true},
		{DriveFakerInput, false},
		{DriveNoop, false},
		{DriveName("unknown"), false},
	}
	for _, c := range cases {
		if got := needsSerialization(c.drive); got != c.want {
			t.Errorf("needsSerialization(%q) = %v, want %v", c.drive, got, c.want)
		}
	}
}
