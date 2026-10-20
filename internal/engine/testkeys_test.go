package engine

import (
	"time"

	"GoKeyMux/internal/input"
)

// Test-only aliases for the input package. The engine tests keep using the
// same short names and KeyCodes values as before the package split.
func mustTestKey(r rune) KeyCodes {
	kc, ok := input.KeyCodesFromRune(r)
	if !ok {
		panic("missing test key")
	}
	return kc
}

var (
	fakerInputKeyA = mustTestKey('a').FakerInput
	fakerInputKeyB = mustTestKey('b').FakerInput
	fakerInputKeyW = mustTestKey('w').FakerInput
	fakerInputKeyS = mustTestKey('s').FakerInput
	fakerInputKeyD = mustTestKey('d').FakerInput
)

const ModLShift = input.ModLShift

// NewNoopDevice keeps the engine tests independent of the input package name.
func NewNoopDevice(latency time.Duration) *NoopDevice {
	return input.NewNoopDevice(latency)
}
