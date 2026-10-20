//go:build realbackend

package engine

import (
	"GoKeyMux/internal/input"
	"log/slog"
	"os"
	"testing"

	"github.com/aiwaki/makc"
)

// The benchmarks in this file inject REAL keyboard input into the focused
// window. They are excluded from the default build and require an explicit
// environment variable on top of the build tag:
//
//	go test -tags realbackend -run XXX -bench BenchmarkRealBackend -benchtime 100x
//
// with GOKEYMUX_REAL_BENCH=1 set.
//
// Before running, focus a scratch window (for example an empty Notepad). The
// benchmark presses and releases F12, which most editors treat as harmless;
// avoid terminals, IDEs and browsers where F12 has a special meaning.
//
// The winputWithInterception backend locates interception.dll relative to the
// test binary, which lives in the Go build cache, so that sub-benchmark will
// normally be skipped unless the DLL tree is copied next to the test binary.
//
// The makc sub-benchmark forces the SendInput backend: on Windows 11 build
// 26200 the auto-selected InjectKeyboardInput backend fails after a few dozen
// injections with "The data area passed to a system call is too small".

func requireRealBench(b *testing.B) {
	b.Helper()
	if os.Getenv("GOKEYMUX_REAL_BENCH") != "1" {
		b.Skip("set GOKEYMUX_REAL_BENCH=1 to run benchmarks that inject real input")
	}
}

// newRealBenchEngine is newBenchEngine with a skip instead of a failure when the
// driver or device is unavailable.
func newRealBenchEngine(b *testing.B, drive DriveName) *Engine {
	b.Helper()
	prevLogger := discardBenchLogs()
	prevCfg := setBenchGlobalConfig(drive, 64, 0)

	e := NewEngine()
	if err := e.StartEngine(); err != nil {
		restoreBenchGlobalConfig(prevCfg)
		slog.SetDefault(prevLogger)
		b.Skipf("%s backend unavailable: %v", drive, err)
	}
	b.Cleanup(func() {
		_ = e.CloseEngine()
		restoreBenchGlobalConfig(prevCfg)
		slog.SetDefault(prevLogger)
	})
	return e
}

// newRealBenchMakcSendInputEngine builds a makc engine around the documented
// SendInput backend instead of the auto-selected one. On Windows 11 build
// 26200, the auto backend (user32!InjectKeyboardInput) starts failing after a
// few dozen injections with "The data area passed to a system call is too
// small", which makes it unusable for a sustained benchmark. Production keeps
// using the auto backend; this only affects the benchmark.
func newRealBenchMakcSendInputEngine(b *testing.B) *Engine {
	b.Helper()

	client, err := makc.Open(
		makc.WithKeyboardInjection(makc.KeyboardInjectionSendInput),
		makc.WithMouseInjection(makc.MouseInjectionSendInput),
	)
	if err != nil {
		b.Skipf("makc SendInput backend unavailable: %v", err)
	}
	b.Logf("makc keyboard backend: %s", client.Keyboard.InjectionBackend())

	prevLogger := discardBenchLogs()
	prevCfg := setBenchGlobalConfig(DriveMakc, 64, 0)

	// Build the engine around the explicit client instead of StartEngine,
	// which would open its own auto-selected client.
	e := NewEngine()
	e.driveName = DriveMakc
	e.driveClient = client
	e.wg.Add(2)
	go e.run()
	go e.runMouse()

	b.Cleanup(func() {
		_ = e.CloseEngine()
		restoreBenchGlobalConfig(prevCfg)
		slog.SetDefault(prevLogger)
	})
	return e
}

// BenchmarkRealBackendPressRelease measures the engine round trip with real
// injection. The numbers include the driver cost, so they are expected to be
// one to three orders of magnitude slower than the noop backend.
func BenchmarkRealBackendPressRelease(b *testing.B) {
	requireRealBench(b)

	key, ok := input.KeyCodesFromName("f12")
	if !ok {
		b.Fatal("key f12 is missing from the table")
	}

	for _, drive := range []DriveName{DriveMakc, DriveFakerInput, DriveWinputWithInterception} {
		b.Run(string(drive), func(b *testing.B) {
			var e *Engine
			if drive == DriveMakc {
				e = newRealBenchMakcSendInputEngine(b)
			} else {
				e = newRealBenchEngine(b, drive)
			}
			benchPressRelease(b, e, key)
		})
	}
}
