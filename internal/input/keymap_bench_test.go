package input

import (
	"testing"

	"github.com/aiwaki/makc"
	"github.com/rpdg/winput"
)

// The benchmarks in this file cover the pure lookup layer. They allocate
// nothing except where noted, so they are cheap to run and useful as a
// regression guard when the key tables change.

func BenchmarkKeyCodesFromName(b *testing.B) {
	cases := []struct {
		name   string
		key    string
		wantOK bool
	}{
		{"lower", "space", true},
		{"mixed_case", "Escape", true},
		{"upper", "F12", true},
		{"miss", "definitely-not-a-key", false},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				kc, ok := KeyCodesFromName(tc.key)
				if ok != tc.wantOK {
					b.Fatalf("KeyCodesFromName(%q) ok=%v, want %v", tc.key, ok, tc.wantOK)
				}
				benchSinkKey = kc
			}
		})
	}
}

func BenchmarkKeyCodesFromRune(b *testing.B) {
	cases := []struct {
		name   string
		r      rune
		wantOK bool
	}{
		{"lower_ascii", 'a', true},
		{"shifted_ascii", 'A', true},
		{"control_enter", '\n', true},
		{"unsupported", '中', false},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				kc, ok := KeyCodesFromRune(tc.r)
				if ok != tc.wantOK {
					b.Fatalf("KeyCodesFromRune(%q) ok=%v, want %v", tc.r, ok, tc.wantOK)
				}
				benchSinkKey = kc
			}
		})
	}
}

func BenchmarkKeyCodesFromRunes(b *testing.B) {
	cases := []struct {
		name   string
		input  string
		wantOK bool
	}{
		{"single", "a", true},
		{"chord3", "was", true},
		{"chord8", "abcdefgh", true},
		{"miss", "abc中", false},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				keys, ok := KeyCodesFromRunes(tc.input)
				if ok != tc.wantOK {
					b.Fatalf("KeyCodesFromRunes(%q) ok=%v, want %v", tc.input, ok, tc.wantOK)
				}
				if len(keys) > 0 {
					benchSinkKey = keys[0]
				}
			}
		})
	}
}

func BenchmarkKeyCodesFromMakc(b *testing.B) {
	a, ok := KeyCodesFromName("space")
	if !ok {
		b.Fatal("key space is missing from the table")
	}
	b.Run("hit", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			kc, ok := KeyCodesFromMakc(a.Makc)
			if !ok {
				b.Fatal("makc lookup miss")
			}
			benchSinkKey = kc
		}
	})
	b.Run("miss", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			kc, ok := KeyCodesFromMakc(makc.Key(0xFFFF))
			if ok {
				b.Fatal("unexpected makc lookup hit")
			}
			benchSinkKey = kc
		}
	})
}

func BenchmarkKeyCodesFromWinput(b *testing.B) {
	a, ok := KeyCodesFromName("space")
	if !ok {
		b.Fatal("key space is missing from the table")
	}
	b.Run("hit", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			kc, ok := KeyCodesFromWinput(a.Winput)
			if !ok {
				b.Fatal("winput lookup miss")
			}
			benchSinkKey = kc
		}
	})
	b.Run("miss", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			kc, ok := KeyCodesFromWinput(winput.Key(0xFFFF))
			if ok {
				b.Fatal("unexpected winput lookup hit")
			}
			benchSinkKey = kc
		}
	})
}

func BenchmarkSamePhysicalKey(b *testing.B) {
	cases := []struct {
		name string
		a, b KeyCodes
		want bool
	}{
		{"same_code", KeyCodes{FakerInput: fakerInputKeyA}, KeyCodes{FakerInput: fakerInputKeyA}, true},
		{"same_code_shifted", KeyCodes{FakerInput: fakerInputKeyA}, KeyCodes{FakerInput: fakerInputKeyA, Modifiers: ModLShift}, true},
		{"different_code", KeyCodes{FakerInput: fakerInputKeyA}, KeyCodes{FakerInput: fakerInputKeyB}, false},
		{"modifier_only", KeyCodes{Modifiers: ModLShift}, KeyCodes{Modifiers: ModLShift}, true},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if got := SamePhysicalKey(tc.a, tc.b); got != tc.want {
					b.Fatalf("SamePhysicalKey(%v, %v) = %v, want %v", tc.a, tc.b, got, tc.want)
				}
			}
		})
	}
}

// benchSinkKey prevents the compiler from eliminating micro-benchmarks whose
// result is otherwise unused.
var benchSinkKey KeyCodes
