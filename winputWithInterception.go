package main

import (
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"

	"github.com/rpdg/winput"
)

var (
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	//procIsWow64Process      = kernel32.NewProc("IsWow64Process")
	procGetNativeSystemInfo = kernel32.NewProc("GetNativeSystemInfo")
)

type systemInfo struct {
	ProcessorArchitecture uint16
	Reserved              uint16
	PageSize              uint32
	// ... 其他字段省略
}

func GetNativeSystemInfo() uint16 {
	var si systemInfo
	procGetNativeSystemInfo.Call(uintptr(unsafe.Pointer(&si)))
	return si.ProcessorArchitecture
}

const (
	PROCESSOR_ARCHITECTURE_INTEL = 0  // x86
	PROCESSOR_ARCHITECTURE_AMD64 = 9  // x64
	PROCESSOR_ARCHITECTURE_ARM64 = 12 // ARM64
)

func WinputWithInterceptionInit() error {
	slog.Debug("WinputWithInterceptionInit")
	arch := GetNativeSystemInfo()
	switch arch {
	case PROCESSOR_ARCHITECTURE_INTEL:
		slog.Debug("is x86")
	case PROCESSOR_ARCHITECTURE_AMD64:
		slog.Debug("is x64")
	case PROCESSOR_ARCHITECTURE_ARM64:
		slog.Debug("is arm64, unsupported")
		return errors.New("not supported")
	default:
		return errors.New("not supported")
	}

	exe, _ := os.Executable()
	baseDir := filepath.Dir(exe)
	switch runtime.GOARCH {
	case "amd64":
		dllPath := filepath.Join(baseDir, "Interception", "library", "x64", "interception.dll")
		winput.SetHIDLibraryPath(dllPath)
	case "386":
		dllPath := filepath.Join(baseDir, "Interception", "library", "x86", "interception.dll")
		winput.SetHIDLibraryPath(dllPath)
	default:
		return fmt.Errorf("unsupported GOARCH: %s", runtime.GOARCH)
	}
	if err := winput.SetBackend(winput.BackendHID); err != nil {
		log.Printf("switch to hid failed: %v", err)
		return err
	}
	return nil

}
