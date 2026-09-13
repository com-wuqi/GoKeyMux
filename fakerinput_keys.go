package main

import (
	"github.com/aiwaki/makc"
	"github.com/rpdg/winput"
)

// Modifier key flags for FakerInputKeyboardReport.ShiftKeyFlags.
const (
	ModLCtrl  byte = 1
	ModLShift byte = 2
	ModLAlt   byte = 4
	ModLGui   byte = 8
	ModRCtrl  byte = 16
	ModRShift byte = 32
	ModRAlt   byte = 64
	ModRGui   byte = 128
)

// FakerInput USB HID keyboard usage codes (HUT 1.22).
const (
	fakerInputKeyNone byte = 0x00

	fakerInputKeyA byte = 0x04
	fakerInputKeyB byte = 0x05
	fakerInputKeyC byte = 0x06
	fakerInputKeyD byte = 0x07
	fakerInputKeyE byte = 0x08
	fakerInputKeyF byte = 0x09
	fakerInputKeyG byte = 0x0A
	fakerInputKeyH byte = 0x0B
	fakerInputKeyI byte = 0x0C
	fakerInputKeyJ byte = 0x0D
	fakerInputKeyK byte = 0x0E
	fakerInputKeyL byte = 0x0F
	fakerInputKeyM byte = 0x10
	fakerInputKeyN byte = 0x11
	fakerInputKeyO byte = 0x12
	fakerInputKeyP byte = 0x13
	fakerInputKeyQ byte = 0x14
	fakerInputKeyR byte = 0x15
	fakerInputKeyS byte = 0x16
	fakerInputKeyT byte = 0x17
	fakerInputKeyU byte = 0x18
	fakerInputKeyV byte = 0x19
	fakerInputKeyW byte = 0x1A
	fakerInputKeyX byte = 0x1B
	fakerInputKeyY byte = 0x1C
	fakerInputKeyZ byte = 0x1D

	fakerInputKey1 byte = 0x1E
	fakerInputKey2 byte = 0x1F
	fakerInputKey3 byte = 0x20
	fakerInputKey4 byte = 0x21
	fakerInputKey5 byte = 0x22
	fakerInputKey6 byte = 0x23
	fakerInputKey7 byte = 0x24
	fakerInputKey8 byte = 0x25
	fakerInputKey9 byte = 0x26
	fakerInputKey0 byte = 0x27

	fakerInputKeyEnter     byte = 0x28
	fakerInputKeyEscape    byte = 0x29
	fakerInputKeyBackspace byte = 0x2A
	fakerInputKeyTab       byte = 0x2B
	fakerInputKeySpace     byte = 0x2C
	fakerInputKeyMinus     byte = 0x2D
	fakerInputKeyEqual     byte = 0x2E
	fakerInputKeyLBracket  byte = 0x2F
	fakerInputKeyRBracket  byte = 0x30
	fakerInputKeyBackslash byte = 0x31
	fakerInputKeySemicolon byte = 0x33
	fakerInputKeyQuote     byte = 0x34
	fakerInputKeyBackquote byte = 0x35
	fakerInputKeyComma     byte = 0x36
	fakerInputKeyPeriod    byte = 0x37
	fakerInputKeySlash     byte = 0x38
	fakerInputKeyCapsLock  byte = 0x39

	fakerInputKeyF1  byte = 0x3A
	fakerInputKeyF2  byte = 0x3B
	fakerInputKeyF3  byte = 0x3C
	fakerInputKeyF4  byte = 0x3D
	fakerInputKeyF5  byte = 0x3E
	fakerInputKeyF6  byte = 0x3F
	fakerInputKeyF7  byte = 0x40
	fakerInputKeyF8  byte = 0x41
	fakerInputKeyF9  byte = 0x42
	fakerInputKeyF10 byte = 0x43
	fakerInputKeyF11 byte = 0x44
	fakerInputKeyF12 byte = 0x45

	fakerInputKeyPrintScreen byte = 0x46
	fakerInputKeyScrollLock  byte = 0x47
	fakerInputKeyInsert      byte = 0x49
	fakerInputKeyHome        byte = 0x4A
	fakerInputKeyPageUp      byte = 0x4B
	fakerInputKeyDelete      byte = 0x4C
	fakerInputKeyEnd         byte = 0x4D
	fakerInputKeyPageDown    byte = 0x4E
	fakerInputKeyRightArrow  byte = 0x4F
	fakerInputKeyLeftArrow   byte = 0x50
	fakerInputKeyDownArrow   byte = 0x51
	fakerInputKeyUpArrow     byte = 0x52
	fakerInputKeyNumLock     byte = 0x53
)

// KeyCodes holds one logical key in all three encodings used across this
// codebase: makc.Key (Windows virtual-key code), winput.Key (Interception scan
// code), and FakerInput (USB HID usage code). Modifiers carries the modifier
// flags (e.g. left shift) needed to produce the mapped rune.
type KeyCodes struct {
	Makc       makc.Key
	Winput     winput.Key
	FakerInput byte
	Modifiers  byte
}

// fakerInputKeyTable is the single source of truth for logical keys (no
// modifiers), pairing all three encodings. It is indexed by both makc.Key and
// winput.Key through keyByMakc / keyByWinput below.
var fakerInputKeyTable = []KeyCodes{
	// Letters
	{makc.KeyA, winput.KeyA, fakerInputKeyA, 0},
	{makc.KeyB, winput.KeyB, fakerInputKeyB, 0},
	{makc.KeyC, winput.KeyC, fakerInputKeyC, 0},
	{makc.KeyD, winput.KeyD, fakerInputKeyD, 0},
	{makc.KeyE, winput.KeyE, fakerInputKeyE, 0},
	{makc.KeyF, winput.KeyF, fakerInputKeyF, 0},
	{makc.KeyG, winput.KeyG, fakerInputKeyG, 0},
	{makc.KeyH, winput.KeyH, fakerInputKeyH, 0},
	{makc.KeyI, winput.KeyI, fakerInputKeyI, 0},
	{makc.KeyJ, winput.KeyJ, fakerInputKeyJ, 0},
	{makc.KeyK, winput.KeyK, fakerInputKeyK, 0},
	{makc.KeyL, winput.KeyL, fakerInputKeyL, 0},
	{makc.KeyM, winput.KeyM, fakerInputKeyM, 0},
	{makc.KeyN, winput.KeyN, fakerInputKeyN, 0},
	{makc.KeyO, winput.KeyO, fakerInputKeyO, 0},
	{makc.KeyP, winput.KeyP, fakerInputKeyP, 0},
	{makc.KeyQ, winput.KeyQ, fakerInputKeyQ, 0},
	{makc.KeyR, winput.KeyR, fakerInputKeyR, 0},
	{makc.KeyS, winput.KeyS, fakerInputKeyS, 0},
	{makc.KeyT, winput.KeyT, fakerInputKeyT, 0},
	{makc.KeyU, winput.KeyU, fakerInputKeyU, 0},
	{makc.KeyV, winput.KeyV, fakerInputKeyV, 0},
	{makc.KeyW, winput.KeyW, fakerInputKeyW, 0},
	{makc.KeyX, winput.KeyX, fakerInputKeyX, 0},
	{makc.KeyY, winput.KeyY, fakerInputKeyY, 0},
	{makc.KeyZ, winput.KeyZ, fakerInputKeyZ, 0},

	// Digits
	{makc.Key0, winput.Key0, fakerInputKey0, 0},
	{makc.Key1, winput.Key1, fakerInputKey1, 0},
	{makc.Key2, winput.Key2, fakerInputKey2, 0},
	{makc.Key3, winput.Key3, fakerInputKey3, 0},
	{makc.Key4, winput.Key4, fakerInputKey4, 0},
	{makc.Key5, winput.Key5, fakerInputKey5, 0},
	{makc.Key6, winput.Key6, fakerInputKey6, 0},
	{makc.Key7, winput.Key7, fakerInputKey7, 0},
	{makc.Key8, winput.Key8, fakerInputKey8, 0},
	{makc.Key9, winput.Key9, fakerInputKey9, 0},

	// Whitespace and common control keys
	{makc.KeyEnter, winput.KeyEnter, fakerInputKeyEnter, 0},
	{makc.KeyEscape, winput.KeyEsc, fakerInputKeyEscape, 0},
	{makc.KeyBackspace, winput.KeyBkSp, fakerInputKeyBackspace, 0},
	{makc.KeyTab, winput.KeyTab, fakerInputKeyTab, 0},
	{makc.KeySpace, winput.KeySpace, fakerInputKeySpace, 0},
	{makc.KeyCapsLock, winput.KeyCaps, fakerInputKeyCapsLock, 0},

	// Symbols
	{makc.KeyMinus, winput.KeyMinus, fakerInputKeyMinus, 0},
	{makc.KeyEquals, winput.KeyEqual, fakerInputKeyEqual, 0},
	{makc.KeyLeftSquareBracket, winput.KeyLBr, fakerInputKeyLBracket, 0},
	{makc.KeyRightSquareBracket, winput.KeyRBr, fakerInputKeyRBracket, 0},
	{makc.KeyBackslash, winput.KeyBackslash, fakerInputKeyBackslash, 0},
	{makc.KeySemicolon, winput.KeySemi, fakerInputKeySemicolon, 0},
	{makc.KeySingleQuote, winput.KeyQuot, fakerInputKeyQuote, 0},
	{makc.KeyBackQuote, winput.KeyTick, fakerInputKeyBackquote, 0},
	{makc.KeyComma, winput.KeyComma, fakerInputKeyComma, 0},
	{makc.KeyDot, winput.KeyDot, fakerInputKeyPeriod, 0},
	{makc.KeySlash, winput.KeySlash, fakerInputKeySlash, 0},

	// Function keys
	{makc.KeyF1, winput.KeyF1, fakerInputKeyF1, 0},
	{makc.KeyF2, winput.KeyF2, fakerInputKeyF2, 0},
	{makc.KeyF3, winput.KeyF3, fakerInputKeyF3, 0},
	{makc.KeyF4, winput.KeyF4, fakerInputKeyF4, 0},
	{makc.KeyF5, winput.KeyF5, fakerInputKeyF5, 0},
	{makc.KeyF6, winput.KeyF6, fakerInputKeyF6, 0},
	{makc.KeyF7, winput.KeyF7, fakerInputKeyF7, 0},
	{makc.KeyF8, winput.KeyF8, fakerInputKeyF8, 0},
	{makc.KeyF9, winput.KeyF9, fakerInputKeyF9, 0},
	{makc.KeyF10, winput.KeyF10, fakerInputKeyF10, 0},
	{makc.KeyF11, winput.KeyF11, fakerInputKeyF11, 0},
	{makc.KeyF12, winput.KeyF12, fakerInputKeyF12, 0},

	// Navigation and editing keys
	{makc.KeyScrollLock, winput.KeyScroll, fakerInputKeyScrollLock, 0},
	{makc.KeyInsert, winput.KeyInsert, fakerInputKeyInsert, 0},
	{makc.KeyHome, winput.KeyHome, fakerInputKeyHome, 0},
	{makc.KeyPageUp, winput.KeyPageUp, fakerInputKeyPageUp, 0},
	{makc.KeyDelete, winput.KeyDelete, fakerInputKeyDelete, 0},
	{makc.KeyEnd, winput.KeyEnd, fakerInputKeyEnd, 0},
	{makc.KeyPageDown, winput.KeyPageDown, fakerInputKeyPageDown, 0},
	{makc.KeyRight, winput.KeyRight, fakerInputKeyRightArrow, 0},
	{makc.KeyLeft, winput.KeyLeft, fakerInputKeyLeftArrow, 0},
	{makc.KeyDown, winput.KeyArrowDown, fakerInputKeyDownArrow, 0},
	{makc.KeyUp, winput.KeyArrowUp, fakerInputKeyUpArrow, 0},
	{makc.KeyNumLock, winput.KeyNumLock, fakerInputKeyNumLock, 0},
}

// keyByMakc indexes fakerInputKeyTable by makc.Key (Windows virtual-key code).
var keyByMakc = func() map[makc.Key]KeyCodes {
	m := make(map[makc.Key]KeyCodes, len(fakerInputKeyTable))
	for _, kc := range fakerInputKeyTable {
		m[kc.Makc] = kc
	}
	return m
}()

// keyByWinput indexes fakerInputKeyTable by winput.Key (Interception scan code).
var keyByWinput = func() map[winput.Key]KeyCodes {
	m := make(map[winput.Key]KeyCodes, len(fakerInputKeyTable))
	for _, kc := range fakerInputKeyTable {
		m[kc.Winput] = kc
	}
	return m
}()

// keyByRune maps a printable ASCII rune (or control rune) to its KeyCodes,
// including the modifier flags required for shifted variants.
var keyByRune = map[rune]KeyCodes{
	'a': {makc.KeyA, winput.KeyA, fakerInputKeyA, 0},
	'A': {makc.KeyA, winput.KeyA, fakerInputKeyA, ModLShift},
	'b': {makc.KeyB, winput.KeyB, fakerInputKeyB, 0},
	'B': {makc.KeyB, winput.KeyB, fakerInputKeyB, ModLShift},
	'c': {makc.KeyC, winput.KeyC, fakerInputKeyC, 0},
	'C': {makc.KeyC, winput.KeyC, fakerInputKeyC, ModLShift},
	'd': {makc.KeyD, winput.KeyD, fakerInputKeyD, 0},
	'D': {makc.KeyD, winput.KeyD, fakerInputKeyD, ModLShift},
	'e': {makc.KeyE, winput.KeyE, fakerInputKeyE, 0},
	'E': {makc.KeyE, winput.KeyE, fakerInputKeyE, ModLShift},
	'f': {makc.KeyF, winput.KeyF, fakerInputKeyF, 0},
	'F': {makc.KeyF, winput.KeyF, fakerInputKeyF, ModLShift},
	'g': {makc.KeyG, winput.KeyG, fakerInputKeyG, 0},
	'G': {makc.KeyG, winput.KeyG, fakerInputKeyG, ModLShift},
	'h': {makc.KeyH, winput.KeyH, fakerInputKeyH, 0},
	'H': {makc.KeyH, winput.KeyH, fakerInputKeyH, ModLShift},
	'i': {makc.KeyI, winput.KeyI, fakerInputKeyI, 0},
	'I': {makc.KeyI, winput.KeyI, fakerInputKeyI, ModLShift},
	'j': {makc.KeyJ, winput.KeyJ, fakerInputKeyJ, 0},
	'J': {makc.KeyJ, winput.KeyJ, fakerInputKeyJ, ModLShift},
	'k': {makc.KeyK, winput.KeyK, fakerInputKeyK, 0},
	'K': {makc.KeyK, winput.KeyK, fakerInputKeyK, ModLShift},
	'l': {makc.KeyL, winput.KeyL, fakerInputKeyL, 0},
	'L': {makc.KeyL, winput.KeyL, fakerInputKeyL, ModLShift},
	'm': {makc.KeyM, winput.KeyM, fakerInputKeyM, 0},
	'M': {makc.KeyM, winput.KeyM, fakerInputKeyM, ModLShift},
	'n': {makc.KeyN, winput.KeyN, fakerInputKeyN, 0},
	'N': {makc.KeyN, winput.KeyN, fakerInputKeyN, ModLShift},
	'o': {makc.KeyO, winput.KeyO, fakerInputKeyO, 0},
	'O': {makc.KeyO, winput.KeyO, fakerInputKeyO, ModLShift},
	'p': {makc.KeyP, winput.KeyP, fakerInputKeyP, 0},
	'P': {makc.KeyP, winput.KeyP, fakerInputKeyP, ModLShift},
	'q': {makc.KeyQ, winput.KeyQ, fakerInputKeyQ, 0},
	'Q': {makc.KeyQ, winput.KeyQ, fakerInputKeyQ, ModLShift},
	'r': {makc.KeyR, winput.KeyR, fakerInputKeyR, 0},
	'R': {makc.KeyR, winput.KeyR, fakerInputKeyR, ModLShift},
	's': {makc.KeyS, winput.KeyS, fakerInputKeyS, 0},
	'S': {makc.KeyS, winput.KeyS, fakerInputKeyS, ModLShift},
	't': {makc.KeyT, winput.KeyT, fakerInputKeyT, 0},
	'T': {makc.KeyT, winput.KeyT, fakerInputKeyT, ModLShift},
	'u': {makc.KeyU, winput.KeyU, fakerInputKeyU, 0},
	'U': {makc.KeyU, winput.KeyU, fakerInputKeyU, ModLShift},
	'v': {makc.KeyV, winput.KeyV, fakerInputKeyV, 0},
	'V': {makc.KeyV, winput.KeyV, fakerInputKeyV, ModLShift},
	'w': {makc.KeyW, winput.KeyW, fakerInputKeyW, 0},
	'W': {makc.KeyW, winput.KeyW, fakerInputKeyW, ModLShift},
	'x': {makc.KeyX, winput.KeyX, fakerInputKeyX, 0},
	'X': {makc.KeyX, winput.KeyX, fakerInputKeyX, ModLShift},
	'y': {makc.KeyY, winput.KeyY, fakerInputKeyY, 0},
	'Y': {makc.KeyY, winput.KeyY, fakerInputKeyY, ModLShift},
	'z': {makc.KeyZ, winput.KeyZ, fakerInputKeyZ, 0},
	'Z': {makc.KeyZ, winput.KeyZ, fakerInputKeyZ, ModLShift},

	'0': {makc.Key0, winput.Key0, fakerInputKey0, 0},
	')': {makc.Key0, winput.Key0, fakerInputKey0, ModLShift},
	'1': {makc.Key1, winput.Key1, fakerInputKey1, 0},
	'!': {makc.Key1, winput.Key1, fakerInputKey1, ModLShift},
	'2': {makc.Key2, winput.Key2, fakerInputKey2, 0},
	'@': {makc.Key2, winput.Key2, fakerInputKey2, ModLShift},
	'3': {makc.Key3, winput.Key3, fakerInputKey3, 0},
	'#': {makc.Key3, winput.Key3, fakerInputKey3, ModLShift},
	'4': {makc.Key4, winput.Key4, fakerInputKey4, 0},
	'$': {makc.Key4, winput.Key4, fakerInputKey4, ModLShift},
	'5': {makc.Key5, winput.Key5, fakerInputKey5, 0},
	'%': {makc.Key5, winput.Key5, fakerInputKey5, ModLShift},
	'6': {makc.Key6, winput.Key6, fakerInputKey6, 0},
	'^': {makc.Key6, winput.Key6, fakerInputKey6, ModLShift},
	'7': {makc.Key7, winput.Key7, fakerInputKey7, 0},
	'&': {makc.Key7, winput.Key7, fakerInputKey7, ModLShift},
	'8': {makc.Key8, winput.Key8, fakerInputKey8, 0},
	'*': {makc.Key8, winput.Key8, fakerInputKey8, ModLShift},
	'9': {makc.Key9, winput.Key9, fakerInputKey9, 0},
	'(': {makc.Key9, winput.Key9, fakerInputKey9, ModLShift},

	'`':  {makc.KeyBackQuote, winput.KeyTick, fakerInputKeyBackquote, 0},
	'~':  {makc.KeyBackQuote, winput.KeyTick, fakerInputKeyBackquote, ModLShift},
	'-':  {makc.KeyMinus, winput.KeyMinus, fakerInputKeyMinus, 0},
	'_':  {makc.KeyMinus, winput.KeyMinus, fakerInputKeyMinus, ModLShift},
	'=':  {makc.KeyEquals, winput.KeyEqual, fakerInputKeyEqual, 0},
	'+':  {makc.KeyEquals, winput.KeyEqual, fakerInputKeyEqual, ModLShift},
	'[':  {makc.KeyLeftSquareBracket, winput.KeyLBr, fakerInputKeyLBracket, 0},
	'{':  {makc.KeyLeftSquareBracket, winput.KeyLBr, fakerInputKeyLBracket, ModLShift},
	']':  {makc.KeyRightSquareBracket, winput.KeyRBr, fakerInputKeyRBracket, 0},
	'}':  {makc.KeyRightSquareBracket, winput.KeyRBr, fakerInputKeyRBracket, ModLShift},
	'\\': {makc.KeyBackslash, winput.KeyBackslash, fakerInputKeyBackslash, 0},
	'|':  {makc.KeyBackslash, winput.KeyBackslash, fakerInputKeyBackslash, ModLShift},
	';':  {makc.KeySemicolon, winput.KeySemi, fakerInputKeySemicolon, 0},
	':':  {makc.KeySemicolon, winput.KeySemi, fakerInputKeySemicolon, ModLShift},
	'\'': {makc.KeySingleQuote, winput.KeyQuot, fakerInputKeyQuote, 0},
	'"':  {makc.KeySingleQuote, winput.KeyQuot, fakerInputKeyQuote, ModLShift},
	',':  {makc.KeyComma, winput.KeyComma, fakerInputKeyComma, 0},
	'<':  {makc.KeyComma, winput.KeyComma, fakerInputKeyComma, ModLShift},
	'.':  {makc.KeyDot, winput.KeyDot, fakerInputKeyPeriod, 0},
	'>':  {makc.KeyDot, winput.KeyDot, fakerInputKeyPeriod, ModLShift},
	'/':  {makc.KeySlash, winput.KeySlash, fakerInputKeySlash, 0},
	'?':  {makc.KeySlash, winput.KeySlash, fakerInputKeySlash, ModLShift},

	' ':    {makc.KeySpace, winput.KeySpace, fakerInputKeySpace, 0},
	'\n':   {makc.KeyEnter, winput.KeyEnter, fakerInputKeyEnter, 0},
	'\r':   {makc.KeyEnter, winput.KeyEnter, fakerInputKeyEnter, 0},
	'\t':   {makc.KeyTab, winput.KeyTab, fakerInputKeyTab, 0},
	'\b':   {makc.KeyBackspace, winput.KeyBkSp, fakerInputKeyBackspace, 0},
	'\x1b': {makc.KeyEscape, winput.KeyEsc, fakerInputKeyEscape, 0},
}

// KeyCodesFromRune returns the three encodings (and required modifiers) for a
// printable ASCII rune or a supported control rune.
func KeyCodesFromRune(r rune) (KeyCodes, bool) {
	kc, ok := keyByRune[r]
	return kc, ok
}

// KeyCodesFromMakc returns the three encodings for a makc.Key (Windows
// virtual-key code).
func KeyCodesFromMakc(k makc.Key) (KeyCodes, bool) {
	kc, ok := keyByMakc[k]
	return kc, ok
}

// KeyCodesFromWinput returns the three encodings for a winput.Key (Interception
// scan code).
func KeyCodesFromWinput(k winput.Key) (KeyCodes, bool) {
	kc, ok := keyByWinput[k]
	return kc, ok
}

// FakerInputKeyFromRune maps a rune to its FakerInput USB HID key code and the
// modifier flags required to produce it.
func FakerInputKeyFromRune(r rune) (code byte, modifiers byte, ok bool) {
	kc, ok := keyByRune[r]
	return kc.FakerInput, kc.Modifiers, ok
}

// FakerInputKeyFromMakc converts a makc.Key (Windows virtual-key code) to a
// FakerInput USB HID key code.
func FakerInputKeyFromMakc(k makc.Key) (code byte, ok bool) {
	kc, ok := keyByMakc[k]
	return kc.FakerInput, ok
}

// FakerInputKeyFromWinput converts a winput.Key (Interception scan code) to a
// FakerInput USB HID key code.
func FakerInputKeyFromWinput(k winput.Key) (code byte, ok bool) {
	kc, ok := keyByWinput[k]
	return kc.FakerInput, ok
}
