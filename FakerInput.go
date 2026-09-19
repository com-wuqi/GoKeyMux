package main

import (
	"errors"
	"fmt"
	"log/slog"

	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// FakerInput report IDs, mirrored from FakerInput/FakerInput/fakerinputcommon.h
const (
	fakerInputReportIDKeyboard          = 0x01
	fakerInputReportIDEnhancedKey       = 0x02
	fakerInputReportIDRelativeMouse     = 0x03
	fakerInputReportIDAbsoluteMouse     = 0x04
	fakerInputReportIDControl           = 0x40
	fakerInputReportIDCheckAPIVersion   = 0x41
	fakerInputReportIDAPIVersionFeature = 0x42

	fakerInputAPIversion = 0x01

	fakerInputKeyboardReportSize = 9
	fakerInputKeyCodeCount       = 6
)

// GUID_DEVINTERFACE_HID, used to enumerate HID collection devices.
var hidInterfaceGUID = windows.GUID{
	Data1: 0x4d1e55b2,
	Data2: 0xf16f,
	Data3: 0x11cf,
	Data4: [8]byte{0x88, 0xcb, 0x00, 0x11, 0x11, 0x00, 0x00, 0x30},
}

// FakerInput HID device attributes and top-level usages from Device.c/Device.h.
const (
	fakerInputVID             = 0xFE0F
	fakerInputPID             = 0x00FF
	fakerInputUsagePage       = 0xFF00
	fakerInputUsageControl    = 0x01 // control collection (report ID 0x40)
	fakerInputUsageAPIVersion = 0x02 // API version collection (report ID 0x41/0x42)
)

var (
	hidDLL         = syscall.NewLazyDLL("hid.dll")
	procGetAttr    = hidDLL.NewProc("HidD_GetAttributes")
	procGetPrep    = hidDLL.NewProc("HidD_GetPreparsedData")
	procGetCaps    = hidDLL.NewProc("HidP_GetCaps")
	procFreePrep   = hidDLL.NewProc("HidD_FreePreparsedData")
	procGetFeature = hidDLL.NewProc("HidD_GetFeature")
)

type hidAttributes struct {
	Size          uint32
	VendorID      uint16
	ProductID     uint16
	VersionNumber uint16
}

// FakerInputDevice is an open handle to the FakerInput virtual HID device.
// It tracks the currently held keyboard state (up to 6 key codes plus a
// per-key modifier mask) so that KeyDown/KeyUp can be issued independently
// without clobbering other held keys.
type FakerInputDevice struct {
	mu     sync.Mutex
	handle windows.Handle

	// Held key slots. Each slot stores one key code and the modifier flags
	// that were requested while that key is held. modifiers reported to the
	// device is the bitwise-OR of keyMods[0:keyCount], which gives correct
	// reference-counted behaviour when several held keys share a modifier.
	keys     [fakerInputKeyCodeCount]byte
	keyMods  [fakerInputKeyCodeCount]byte
	keyCount int

	// reportBuf is a reusable control report buffer. It is only touched while
	// mu is held, so it needs no further synchronization and avoids a heap
	// allocation on every key event.
	reportBuf [65]byte
}

// FakerInputInit finds and opens the FakerInput control collection, the handle
// used to inject keyboard reports.
func FakerInputInit() (*FakerInputDevice, error) {
	slog.Debug("FakerInputInit")
	handle, err := openFakerInputCollection(fakerInputUsageControl)
	if err != nil {
		return nil, err
	}
	return &FakerInputDevice{handle: handle}, nil
}

// openFakerInputCollection enumerates HID devices, matches the FakerInput
// VID/PID, and returns an open handle to the top-level collection whose usage
// equals wantUsage.
func openFakerInputCollection(wantUsage uint16) (windows.Handle, error) {
	paths, err := windows.CM_Get_Device_Interface_List(
		"",
		&hidInterfaceGUID,
		windows.CM_GET_DEVICE_INTERFACE_LIST_PRESENT,
	)
	if err != nil {
		return windows.InvalidHandle, fmt.Errorf("enumerate HID device interfaces: %w", err)
	}

	for _, p := range paths {
		path, err := windows.UTF16PtrFromString(p)
		if err != nil {
			continue
		}
		handle, err := windows.CreateFile(
			path,
			windows.GENERIC_READ|windows.GENERIC_WRITE,
			windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
			nil,
			windows.OPEN_EXISTING,
			0,
			0,
		)
		if err != nil {
			continue
		}

		attrs, ok := getHIDAttributes(handle)
		if !ok || attrs.VendorID != fakerInputVID || attrs.ProductID != fakerInputPID {
			windows.CloseHandle(handle)
			continue
		}

		usagePage, usage, ok := getHIDTopLevelUsage(handle)
		if ok && usagePage == fakerInputUsagePage && usage == wantUsage {
			return handle, nil
		}

		windows.CloseHandle(handle)
	}

	return windows.InvalidHandle, errors.New("FakerInput device collection not found")
}

func getHIDAttributes(h windows.Handle) (hidAttributes, bool) {
	var a hidAttributes
	a.Size = uint32(unsafe.Sizeof(a))
	r, _, _ := procGetAttr.Call(uintptr(h), uintptr(unsafe.Pointer(&a)))
	return a, r != 0
}

func getHIDTopLevelUsage(h windows.Handle) (usagePage, usage uint16, ok bool) {
	var prep uintptr
	r, _, _ := procGetPrep.Call(uintptr(h), uintptr(unsafe.Pointer(&prep)))
	if r == 0 || prep == 0 {
		return 0, 0, false
	}
	defer procFreePrep.Call(prep)

	var caps [64]byte
	status, _, _ := procGetCaps.Call(prep, uintptr(unsafe.Pointer(&caps[0])))
	if status != 0x00110000 { // HIDP_STATUS_SUCCESS
		return 0, 0, false
	}

	usage = *(*uint16)(unsafe.Pointer(&caps[0]))
	usagePage = *(*uint16)(unsafe.Pointer(&caps[2]))
	return usagePage, usage, true
}

// Close closes the device handle.
func (d *FakerInputDevice) Close() error {
	if d == nil || d.handle == windows.InvalidHandle {
		return nil
	}
	err := windows.CloseHandle(d.handle)
	d.handle = windows.InvalidHandle
	return err
}

// CheckAPIVersion reads the API version feature report (0x42) from the
// FakerInput API version collection as a handshake, returning the version.
func (d *FakerInputDevice) CheckAPIVersion() (uint32, error) {
	handle, err := openFakerInputCollection(fakerInputUsageAPIVersion)
	if err != nil {
		return 0, err
	}
	defer windows.CloseHandle(handle)

	report := make([]byte, 65) // feature report: report ID + 64 data bytes
	report[0] = fakerInputReportIDAPIVersionFeature
	r, _, _ := procGetFeature.Call(
		uintptr(handle),
		uintptr(unsafe.Pointer(&report[0])),
		uintptr(len(report)),
	)
	if r == 0 {
		return 0, errors.New("HidD_GetFeature failed")
	}

	version := uint32(report[4]) | uint32(report[5])<<8 | uint32(report[6])<<16 | uint32(report[7])<<24
	return version, nil
}

// KeyDown presses a single key and keeps it held. The key is added to the
// device's held-key set, so repeatedly calling KeyDown with different codes
// accumulates keys (e.g. KeyDown(a) then KeyDown(b) holds both). modifiers
// flags are held for this key while it is down; pressing an already-held code
// ORs the new modifiers into it.
func (d *FakerInputDevice) KeyDown(code byte, modifiers byte) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.keyDownLocked(code, modifiers)
}

// KeyUp releases a single previously held key. If modifiers is non-zero, only
// those modifier flags are cleared from the key's slot; the key itself remains
// held until its modifier mask is empty. modifiers must mirror the flags that
// were held by the matching KeyDown.
func (d *FakerInputDevice) KeyUp(code byte, modifiers byte) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.keyUpLocked(code, modifiers)
}

// ReleaseAll releases every held key and modifier at once.
func (d *FakerInputDevice) ReleaseAll() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.releaseAllLocked()
}

// SetKeys replaces the held-key state with the given keys (up to 6) and a
// single modifier mask applied to all of them. Codes beyond the first 6 are
// ignored; an empty slice releases all keys.
func (d *FakerInputDevice) SetKeys(codes []byte, modifiers byte) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.setKeysLocked(codes, modifiers)
}

// Tap presses and releases a single key.
func (d *FakerInputDevice) Tap(code byte, modifiers byte) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.tapLocked(code, modifiers)
}

// TypeText types the printable ASCII characters in s. Non-printable characters
// are ignored; newline is mapped to Enter.
func (d *FakerInputDevice) TypeText(s string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, r := range s {
		code, mod, ok := FakerInputKeyFromRune(r)
		if !ok {
			continue
		}
		if err := d.tapLocked(code, mod); err != nil {
			return err
		}
	}
	return nil
}

func (d *FakerInputDevice) keyDownLocked(code byte, modifiers byte) error {
	if code != 0 {
		for i := 0; i < d.keyCount; i++ {
			if d.keys[i] == code {
				d.keyMods[i] |= modifiers
				return d.sendHeldLocked()
			}
		}
		if d.keyCount < fakerInputKeyCodeCount {
			d.keys[d.keyCount] = code
			d.keyMods[d.keyCount] = modifiers
			d.keyCount++
		}
	}
	return d.sendHeldLocked()
}

func (d *FakerInputDevice) keyUpLocked(code byte, modifiers byte) error {
	if code != 0 {
		for i := 0; i < d.keyCount; i++ {
			if d.keys[i] == code {
				d.keyMods[i] &^= modifiers
				if d.keyMods[i] == 0 {
					d.keyCount--
					d.keys[i] = d.keys[d.keyCount]
					d.keyMods[i] = d.keyMods[d.keyCount]
				}
				break
			}
		}
	}
	return d.sendHeldLocked()
}

func (d *FakerInputDevice) setKeysLocked(codes []byte, modifiers byte) error {
	d.keyCount = 0
	for _, c := range codes {
		if c == 0 {
			continue
		}
		if d.keyCount >= fakerInputKeyCodeCount {
			break
		}
		d.keys[d.keyCount] = c
		d.keyMods[d.keyCount] = modifiers
		d.keyCount++
	}
	return d.sendHeldLocked()
}

func (d *FakerInputDevice) releaseAllLocked() error {
	return d.setKeysLocked(nil, 0)
}

func (d *FakerInputDevice) tapLocked(code byte, modifiers byte) error {
	if err := d.keyDownLocked(code, modifiers); err != nil {
		return err
	}
	return d.keyUpLocked(code, modifiers)
}

// heldState returns the effective keyboard state: the OR of all held keys'
// modifier masks, and the ordered held key codes (trailing slots zeroed).
func (d *FakerInputDevice) heldState() (mods byte, keys [fakerInputKeyCodeCount]byte) {
	copy(keys[:], d.keys[:d.keyCount])
	for i := 0; i < d.keyCount; i++ {
		mods |= d.keyMods[i]
	}
	return mods, keys
}

// sendHeldLocked assembles the current held-key state and writes it to the
// device.
func (d *FakerInputDevice) sendHeldLocked() error {
	mods, keys := d.heldState()
	return d.sendKeyboardReportLocked(mods, keys)
}

// fillKeyboardReport assembles a 9-byte keyboard input report into dst:
// report ID (0x01), shift-key flags, reserved byte, then up to 6 key codes.
func fillKeyboardReport(dst *[fakerInputKeyboardReportSize]byte, modifiers byte, keys [fakerInputKeyCodeCount]byte) {
	dst[0] = fakerInputReportIDKeyboard
	dst[1] = modifiers
	dst[2] = 0
	copy(dst[3:], keys[:])
}

// fillControlReport wraps inner in a full 65-byte control output report
// (CONTROL_REPORT_SIZE = 0x41). Layout, matching FakerInputDll:
//
//	[0]     = ReportID (0x40), also serves as the hidclass routing report ID
//	[1]     = ReportLength (len(inner))
//	[2.]   = inner report
//	rest    = zero padding
func fillControlReport(dst *[65]byte, inner []byte) {
	dst[0] = fakerInputReportIDControl
	dst[1] = byte(len(inner))
	copy(dst[2:], inner)
	clear(dst[2+len(inner):])
}

// sendKeyboardReportLocked writes a keyboard input report into the device's
// reusable report buffer and injects it via the control report. It performs no
// heap allocation.
func (d *FakerInputDevice) sendKeyboardReportLocked(modifiers byte, keys [fakerInputKeyCodeCount]byte) error {
	if d == nil || d.handle == windows.InvalidHandle {
		return errors.New("FakerInput device is not open")
	}

	var inner [fakerInputKeyboardReportSize]byte
	fillKeyboardReport(&inner, modifiers, keys)
	fillControlReport(&d.reportBuf, inner[:])

	var written uint32
	if err := windows.WriteFile(d.handle, d.reportBuf[:], &written, nil); err != nil {
		return fmt.Errorf("write FakerInput control report: %w", err)
	}
	return nil
}

func putUint32LE(b []byte, v uint32) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
}
