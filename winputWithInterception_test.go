package main

import (
	"testing"
	"unsafe"
)

// TestSystemInfoSize guards the SYSTEM_INFO layout against future truncation:
// GetNativeSystemInfo writes the entire structure, so an incomplete Go struct
// would overflow the stack buffer.
func TestSystemInfoSize(t *testing.T) {
	var want uintptr
	switch unsafe.Sizeof(uintptr(0)) {
	case 4:
		want = 36
	case 8:
		want = 48
	default:
		t.Skipf("unexpected pointer size %d", unsafe.Sizeof(uintptr(0)))
	}
	if got := unsafe.Sizeof(systemInfo{}); got != want {
		t.Fatalf("systemInfo size = %d, want %d (full SYSTEM_INFO layout)", got, want)
	}
}
