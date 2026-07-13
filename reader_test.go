package ansi

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"testing/iotest"
)

// sig renders an S as a stable signature for comparison (Code, Type, Params).
func sig(s *S) string {
	return fmt.Sprintf("%q/%s/%q", string(s.Code), s.Type, s.Params)
}

// coalesce merges adjacent plain-text sequences (Type == "") into one.  A
// streaming Reader legitimately splits a text run across reads, so this
// normalizes both sides before comparing to DecodeAll, which never splits it.
func coalesce(ss []*S) []*S {
	var out []*S
	for _, s := range ss {
		if s.Type == "" && len(out) > 0 && out[len(out)-1].Type == "" {
			prev := out[len(out)-1]
			merged := *prev
			merged.Code = prev.Code + s.Code
			out[len(out)-1] = &merged
			continue
		}
		out = append(out, s)
	}
	return out
}

func readAll(rd *Reader) ([]*S, error) {
	var ss []*S
	for {
		s, err := rd.Next()
		if s != nil {
			ss = append(ss, s)
		}
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			return ss, err
		}
	}
}

// TestReaderMatchesDecodeAll checks that streaming a well-formed input --
// including one byte at a time, which forces every sequence to span reads --
// yields exactly the sequences DecodeAll produces on the whole buffer.
func TestReaderMatchesDecodeAll(t *testing.T) {
	for _, d := range []Decoder{{}, {UTF8: true}, {C0: true}} {
		for _, in := range []string{
			"",
			"hello world",
			"\033[1;2Hmoved\033[0m done",
			"a\033Bb\033[Ac\033[4Ad",
			"\033]0;title\a after bel",
			"\033]0;title\033\\ after st",
			"\033_APC string\033\\rest",
			"café \033[31mrouge\033[0m", // multibyte rune
			"tab\tnewline\nreturn\r",    // C0 chars
			"pre\233Amid\233Bpost",      // single-byte CSI
		} {
			want := coalesce(d.DecodeAll([]byte(in)))

			for _, tc := range []struct {
				name string
				r    io.Reader
			}{
				{"whole", bytes.NewReader([]byte(in))},
				{"onebyte", iotest.OneByteReader(bytes.NewReader([]byte(in)))},
				{"half", iotest.HalfReader(bytes.NewReader([]byte(in)))},
			} {
				got, err := readAll(d.NewReader(tc.r))
				if err != nil {
					t.Errorf("d=%+v in=%q [%s]: unexpected error %v", d, in, tc.name, err)
					continue
				}
				got = coalesce(got)
				if len(got) != len(want) {
					t.Errorf("d=%+v in=%q [%s]: got %d sequences, want %d", d, in, tc.name, len(got), len(want))
					continue
				}
				for i := range want {
					if sig(got[i]) != sig(want[i]) {
						t.Errorf("d=%+v in=%q [%s]: seq %d = %s, want %s", d, in, tc.name, i, sig(got[i]), sig(want[i]))
					}
				}
			}
		}
	}
}

// TestReaderTruncated checks that a sequence truncated by end-of-stream
// surfaces the same error Decode would give, rather than being lost or
// causing the Reader to spin.
func TestReaderTruncated(t *testing.T) {
	for _, tt := range []struct {
		in  string
		err error
	}{
		{"abc\033", LoneEscape},
		{"\033[1;2", IncompleteCSI},
		{"\033[1;2 ", IncompleteCSI}, // ends in an intermediate byte
		{"\033_no terminator here", NoST},
	} {
		// One byte at a time to prove the incompleteness is only decided
		// at EOF, not at an interior read boundary.
		rd := NewReader(iotest.OneByteReader(strings.NewReader(tt.in)))
		var last error
		for {
			_, err := rd.Next()
			if err != nil {
				last = err
				break
			}
		}
		if !errors.Is(last, tt.err) {
			t.Errorf("%q: final error = %v, want %v", tt.in, last, tt.err)
		}
	}
}

// TestReaderPartialUTF8Tail checks that a stream ending in an incomplete
// UTF-8 rune (with UTF8 enabled) hands the trailing bytes back as text rather
// than dropping them or reporting an error.
func TestReaderPartialUTF8Tail(t *testing.T) {
	const in = "hello\xe4" // 0xe4 begins a 3-byte rune that never completes
	rd := Decoder{UTF8: true}.NewReader(iotest.OneByteReader(strings.NewReader(in)))
	got, err := readAll(rd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var b strings.Builder
	for _, s := range got {
		b.WriteString(s.String())
	}
	if b.String() != in {
		t.Errorf("reconstructed %q, want %q (no bytes may be dropped)", b.String(), in)
	}
}

// TestReaderEOF checks that Next returns io.EOF once, repeatedly, on an empty
// or drained stream.
func TestReaderEOF(t *testing.T) {
	rd := NewReader(strings.NewReader(""))
	for i := 0; i < 3; i++ {
		s, err := rd.Next()
		if s != nil || err != io.EOF {
			t.Fatalf("call %d: got (%v, %v), want (nil, EOF)", i, s, err)
		}
	}
}
