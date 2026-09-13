package main

import (
	"errors"
	"fmt"
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
type FakerInputDevice struct {
	mu     sync.Mutex
	handle windows.Handle
}

// FakerInputInit finds and opens the FakerInput control collection, the handle
// used to inject keyboard reports.
func FakerInputInit() (*FakerInputDevice, error) {
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

// KeyDown presses a single key with optional modifier flags held.
func (d *FakerInputDevice) KeyDown(code byte, modifiers byte) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.keyDownLocked(code, modifiers)
}

// KeyUp releases all keys and modifiers.
func (d *FakerInputDevice) KeyUp() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.keyUpLocked()
}

// SetKeys holds the given keys (up to 6) and modifiers simultaneously. Codes
// beyond the first 6 are ignored; an empty slice releases all keys. This is the
// primitive for multi-key combos and for releasing keys one at a time.
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
	return d.setKeysLocked([]byte{code}, modifiers)
}

func (d *FakerInputDevice) keyUpLocked() error {
	return d.setKeysLocked(nil, 0)
}

func (d *FakerInputDevice) setKeysLocked(codes []byte, modifiers byte) error {
	var keys [fakerInputKeyCodeCount]byte
	n := min(len(codes), fakerInputKeyCodeCount)
	copy(keys[:], codes[:n])
	return d.sendKeyboardReport(modifiers, keys)
}

func (d *FakerInputDevice) tapLocked(code byte, modifiers byte) error {
	if err := d.keyDownLocked(code, modifiers); err != nil {
		return err
	}
	return d.keyUpLocked()
}

// buildKeyboardReport assembles the 9-byte keyboard input report:
// report ID (0x01), shift-key flags, reserved byte, then up to 6 key codes.
func buildKeyboardReport(modifiers byte, keys [fakerInputKeyCodeCount]byte) []byte {
	inner := make([]byte, fakerInputKeyboardReportSize)
	inner[0] = fakerInputReportIDKeyboard
	inner[1] = modifiers
	inner[2] = 0
	copy(inner[3:], keys[:])
	return inner
}

// sendKeyboardReport injects a 9-byte keyboard input report via the control report.
func (d *FakerInputDevice) sendKeyboardReport(modifiers byte, keys [fakerInputKeyCodeCount]byte) error {
	return d.writeControlReport(buildKeyboardReport(modifiers, keys))
}

// buildControlReport wraps inner in a full 65-byte control output report
// (CONTROL_REPORT_SIZE = 0x41). Layout, matching FakerInputDll:
//
//	[0]     = ReportID (0x40), also serves as the hidclass routing report ID
//	[1]     = ReportLength (len(inner))
//	[2..]   = inner report
//	rest    = zero padding
func buildControlReport(inner []byte) []byte {
	report := make([]byte, 65)
	report[0] = fakerInputReportIDControl
	report[1] = byte(len(inner))
	copy(report[2:], inner)
	return report
}

// writeControlReport wraps inner in a control report and writes it to the device.
func (d *FakerInputDevice) writeControlReport(inner []byte) error {
	if d == nil || d.handle == windows.InvalidHandle {
		return errors.New("FakerInput device is not open")
	}
	if len(inner) > 62 {
		return fmt.Errorf("FakerInput inner report too large: %d", len(inner))
	}

	var written uint32
	if err := windows.WriteFile(d.handle, buildControlReport(inner), &written, nil); err != nil {
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
