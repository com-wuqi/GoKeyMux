package main

import (
	"bytes"
	"testing"

	"github.com/aiwaki/makc"
	"github.com/rpdg/winput"
)

func TestPutUint32LE(t *testing.T) {
	b := make([]byte, 4)
	putUint32LE(b, 0x01020304)
	if !bytes.Equal(b, []byte{0x04, 0x03, 0x02, 0x01}) {
		t.Fatalf("unexpected little-endian encoding: %v", b)
	}
}

func TestWriteControlReportLayout(t *testing.T) {
	inner := []byte{0x01, 0x00, 0x00, 0x04, 0x00, 0x00, 0x00, 0x00, 0x00}
	report := buildControlReport(inner)
	if len(report) != 65 {
		t.Fatalf("unexpected report length: %d, want 65", len(report))
	}
	if report[0] != fakerInputReportIDControl {
		t.Fatalf("unexpected report ID: %#x, want %#x", report[0], fakerInputReportIDControl)
	}
	if report[1] != byte(len(inner)) {
		t.Fatalf("unexpected ReportLength: %d, want %d", report[1], len(inner))
	}
	if !bytes.Equal(report[2:2+len(inner)], inner) {
		t.Fatalf("inner report mismatch: %v", report[2:])
	}
}

// TestKeyCodesFromRune verifies a representative sample of runes against
// independently known values for all three encodings. Values are Windows VK,
// Interception scan code, and USB HID usage code respectively.
func TestKeyCodesFromRune(t *testing.T) {
	cases := []struct {
		r    rune
		want KeyCodes
	}{
		{'a', KeyCodes{0x41, 0x1E, 0x04, 0}},
		{'A', KeyCodes{0x41, 0x1E, 0x04, ModLShift}},
		{'z', KeyCodes{0x5A, 0x2C, 0x1D, 0}},
		{'Z', KeyCodes{0x5A, 0x2C, 0x1D, ModLShift}},
		{'0', KeyCodes{0x30, 0x0B, 0x27, 0}},
		{'9', KeyCodes{0x39, 0x0A, 0x26, 0}},
		{'1', KeyCodes{0x31, 0x02, 0x1E, 0}},
		{'!', KeyCodes{0x31, 0x02, 0x1E, ModLShift}},
		{' ', KeyCodes{0x20, 0x39, 0x2C, 0}},
		{'\n', KeyCodes{0x0D, 0x1C, 0x28, 0}},
		{'\r', KeyCodes{0x0D, 0x1C, 0x28, 0}},
		{'\t', KeyCodes{0x09, 0x0F, 0x2B, 0}},
		{'\b', KeyCodes{0x08, 0x0E, 0x2A, 0}},
		{'\x1b', KeyCodes{0x1B, 0x01, 0x29, 0}},
		{'-', KeyCodes{0xBD, 0x0C, 0x2D, 0}},
		{'_', KeyCodes{0xBD, 0x0C, 0x2D, ModLShift}},
		{'/', KeyCodes{0xBF, 0x35, 0x38, 0}},
		{'?', KeyCodes{0xBF, 0x35, 0x38, ModLShift}},
	}
	for _, c := range cases {
		got, ok := KeyCodesFromRune(c.r)
		if !ok {
			t.Errorf("KeyCodesFromRune(%q): unexpected not-ok", c.r)
			continue
		}
		if got != c.want {
			t.Errorf("KeyCodesFromRune(%q) = %+v, want %+v", c.r, got, c.want)
		}
	}

	if _, ok := KeyCodesFromRune('é'); ok {
		t.Errorf("KeyCodesFromRune('é') should not be ok")
	}
}

func TestKeyCodesFromMakc(t *testing.T) {
	cases := []struct {
		in   makc.Key
		want KeyCodes
	}{
		{makc.KeyA, KeyCodes{makc.KeyA, winput.KeyA, fakerInputKeyA, 0}},
		{makc.KeyF1, KeyCodes{makc.KeyF1, winput.KeyF1, fakerInputKeyF1, 0}},
		{makc.KeyLeft, KeyCodes{makc.KeyLeft, winput.KeyLeft, fakerInputKeyLeftArrow, 0}},
		{makc.KeyNumLock, KeyCodes{makc.KeyNumLock, winput.KeyNumLock, fakerInputKeyNumLock, 0}},
	}
	for _, c := range cases {
		got, ok := KeyCodesFromMakc(c.in)
		if !ok || got != c.want {
			t.Errorf("KeyCodesFromMakc(%#x) = (%+v, %v), want (%+v, true)", c.in, got, ok, c.want)
		}
	}
	if _, ok := KeyCodesFromMakc(makc.Key(0xFFFF)); ok {
		t.Errorf("KeyCodesFromMakc(0xFFFF) should not be ok")
	}
}

func TestKeyCodesFromWinput(t *testing.T) {
	cases := []struct {
		in   winput.Key
		want KeyCodes
	}{
		{winput.KeyA, KeyCodes{makc.KeyA, winput.KeyA, fakerInputKeyA, 0}},
		{winput.KeyF1, KeyCodes{makc.KeyF1, winput.KeyF1, fakerInputKeyF1, 0}},
		{winput.KeyLeft, KeyCodes{makc.KeyLeft, winput.KeyLeft, fakerInputKeyLeftArrow, 0}},
		{winput.KeyNumLock, KeyCodes{makc.KeyNumLock, winput.KeyNumLock, fakerInputKeyNumLock, 0}},
	}
	for _, c := range cases {
		got, ok := KeyCodesFromWinput(c.in)
		if !ok || got != c.want {
			t.Errorf("KeyCodesFromWinput(%#x) = (%+v, %v), want (%+v, true)", c.in, got, ok, c.want)
		}
	}
	if _, ok := KeyCodesFromWinput(winput.Key(0xFFFF)); ok {
		t.Errorf("KeyCodesFromWinput(0xFFFF) should not be ok")
	}
}

// TestKeyCodesConsistency cross-checks every entry against the other two
// encodings, guaranteeing that the rune table and the makc/winput index tables
// all agree on the same logical key.
func TestKeyCodesConsistency(t *testing.T) {
	if len(keyByMakc) != len(fakerInputKeyTable) {
		t.Fatalf("keyByMakc has %d entries, want %d (duplicate VK codes?)", len(keyByMakc), len(fakerInputKeyTable))
	}
	if len(keyByWinput) != len(fakerInputKeyTable) {
		t.Fatalf("keyByWinput has %d entries, want %d (duplicate scan codes?)", len(keyByWinput), len(fakerInputKeyTable))
	}

	for _, kc := range fakerInputKeyTable {
		if got, ok := KeyCodesFromMakc(kc.Makc); !ok || got.Winput != kc.Winput || got.FakerInput != kc.FakerInput {
			t.Errorf("table entry makc=%#x inconsistent with keyByMakc: got (%+v, %v)", kc.Makc, got, ok)
		}
		if got, ok := KeyCodesFromWinput(kc.Winput); !ok || got.Makc != kc.Makc || got.FakerInput != kc.FakerInput {
			t.Errorf("table entry winput=%#x inconsistent with keyByWinput: got (%+v, %v)", kc.Winput, got, ok)
		}
	}

	for r, kc := range keyByRune {
		if got, ok := KeyCodesFromMakc(kc.Makc); !ok || got.Winput != kc.Winput || got.FakerInput != kc.FakerInput {
			t.Errorf("rune %q: makc=%#x inconsistent with keyByMakc (got %+v, %v)", r, kc.Makc, got, ok)
		}
		if got, ok := KeyCodesFromWinput(kc.Winput); !ok || got.Makc != kc.Makc || got.FakerInput != kc.FakerInput {
			t.Errorf("rune %q: winput=%#x inconsistent with keyByWinput (got %+v, %v)", r, kc.Winput, got, ok)
		}
	}
}

// TestKeyTableReference verifies every mapped rune against an independently
// hard-coded reference of Windows VK codes, Interception scan codes, and USB
// HID usage codes. This catches wrong pairings or off-by-one HID values that
// the self-consistency test cannot.
func TestKeyTableReference(t *testing.T) {
	check := func(r rune, vk, scan, hid, mod uint32) {
		t.Helper()
		want := KeyCodes{makc.Key(vk), winput.Key(scan), byte(hid), byte(mod)}
		if got, ok := KeyCodesFromRune(r); !ok || got != want {
			t.Errorf("KeyCodesFromRune(%q) = (%+v, %v), want %+v", r, got, ok, want)
		}
	}

	// Letters: VK 0x41+i, HID 0x04+i. Interception scan codes are non-contiguous.
	letterScan := [...]uint32{
		0x1E, 0x30, 0x2E, 0x20, 0x12, 0x21, 0x22, 0x23, 0x17, 0x24, 0x25, 0x26, 0x32,
		0x31, 0x18, 0x19, 0x10, 0x13, 0x1F, 0x14, 0x16, 0x2F, 0x11, 0x2D, 0x15, 0x2C,
	}
	for i := range 26 {
		vk := uint32(0x41 + i)
		hid := uint32(0x04 + i)
		check(rune('a'+i), vk, letterScan[i], hid, 0)
		check(rune('A'+i), vk, letterScan[i], hid, uint32(ModLShift))
	}

	// Digits 1-9: VK 0x31+i, scan 0x02+i, HID 0x1E+i. '0' is the exception.
	shiftedDigits := []rune{')', '!', '@', '#', '$', '%', '^', '&', '*', '('}
	for i := 1; i <= 9; i++ {
		vk := uint32(0x31 + i - 1)
		scan := uint32(0x02 + i - 1)
		hid := uint32(0x1E + i - 1)
		check(rune('0'+i), vk, scan, hid, 0)
		check(shiftedDigits[i], vk, scan, hid, uint32(ModLShift))
	}
	check('0', 0x30, 0x0B, 0x27, 0)
	check(')', 0x30, 0x0B, 0x27, uint32(ModLShift))

	// Symbols.
	symbols := []struct {
		r       rune
		vk, sc  uint32
		hid     uint32
		shifted bool
	}{
		{'`', 0xC0, 0x29, 0x35, false},
		{'-', 0xBD, 0x0C, 0x2D, false},
		{'=', 0xBB, 0x0D, 0x2E, false},
		{'[', 0xDB, 0x1A, 0x2F, false},
		{']', 0xDD, 0x1B, 0x30, false},
		{'\\', 0xDC, 0x2B, 0x31, false},
		{';', 0xBA, 0x27, 0x33, false},
		{'\'', 0xDE, 0x28, 0x34, false},
		{',', 0xBC, 0x33, 0x36, false},
		{'.', 0xBE, 0x34, 0x37, false},
		{'/', 0xBF, 0x35, 0x38, false},
	}
	shifted := []rune{'~', '_', '+', '{', '}', '|', ':', '"', '<', '>', '?'}
	for i, s := range symbols {
		mod := uint32(0)
		check(s.r, s.vk, s.sc, s.hid, mod)
		check(shifted[i], s.vk, s.sc, s.hid, uint32(ModLShift))
	}

	// Control runes.
	check(' ', 0x20, 0x39, 0x2C, 0)
	check('\n', 0x0D, 0x1C, 0x28, 0)
	check('\r', 0x0D, 0x1C, 0x28, 0)
	check('\t', 0x09, 0x0F, 0x2B, 0)
	check('\b', 0x08, 0x0E, 0x2A, 0)
	check('\x1b', 0x1B, 0x01, 0x29, 0)

	// Every mapped rune must be covered by the reference above; otherwise the
	// reference check is incomplete.
	if len(keyByRune) != 26*2+10*2+11*2+6 {
		t.Errorf("keyByRune has %d entries, reference covers %d", len(keyByRune), 26*2+10*2+11*2+6)
	}
}

// TestFakerInputKeyDelegates verifies the FakerInput-only helpers derive from
// the unified KeyCodes tables.
func TestFakerInputKeyDelegates(t *testing.T) {
	code, mod, ok := FakerInputKeyFromRune('A')
	if !ok || code != fakerInputKeyA || mod != ModLShift {
		t.Errorf("FakerInputKeyFromRune('A') = (%#x, %#x, %v), want (%#x, %#x, true)", code, mod, ok, fakerInputKeyA, ModLShift)
	}

	code, ok = FakerInputKeyFromMakc(makc.KeyEnter)
	if !ok || code != fakerInputKeyEnter {
		t.Errorf("FakerInputKeyFromMakc(Enter) = (%#x, %v), want (%#x, true)", code, ok, fakerInputKeyEnter)
	}

	code, ok = FakerInputKeyFromWinput(winput.KeyEsc)
	if !ok || code != fakerInputKeyEscape {
		t.Errorf("FakerInputKeyFromWinput(Esc) = (%#x, %v), want (%#x, true)", code, ok, fakerInputKeyEscape)
	}
}
