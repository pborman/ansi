// Copyright 2014 Google Inc.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ansi

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestDecode(t *testing.T) {
	for _, tt := range []struct {
		in  string
		rem string
		lu  *Sequence
		out *S
		s   string // what out.String should return, if not in.
		err error
	}{
		{
			in: "abc",
			out: &S{
				Code: "abc",
			},
		},
		{
			in: "\033[1;23;456h",
			out: &S{
				Code: "\033[h",
				Type: "CSI",
				Params: []string{
					"1",
					"23",
					"456",
				},
			},
			lu: &SM_,
		},
		{
			in: "\033B",
			out: &S{
				Code: "\033B",
				Type: "C1",
			},
			lu: &BPH_,
		},
		{
			in: "\202", // One byte version of "\033B"
			s:  "\033B",
			out: &S{
				Code: "\033B",
				Type: "C1",
			},
			lu: &BPH_,
		},
		{
			in: "\033[1;23;456",
			out: &S{
				Code: "\033[",
				Type: "CSI",
				Params: []string{
					"1",
					"23",
					"456",
				},
			},
			lu:  &CSI_,
			err: IncompleteCSI,
		},
		{
			in: "\033[1;23;456 ",
			out: &S{
				Code: "\033[ ",
				Type: "CSI",
				Params: []string{
					"1",
					"23",
					"456",
				},
			},
			err: IncompleteCSI,
		},
		{
			in: "\033[A",
			s:  "\033[1A",
			out: &S{
				Code:   "\033[A",
				Type:   "CSI",
				Params: []string{"1"},
			},
			lu: &CUU_,
		},
		{
			in: "\033[42A",
			out: &S{
				Code:   "\033[A",
				Type:   "CSI",
				Params: []string{"42"},
			},
			lu: &CUU_,
		},
		{
			in: "\033[4;2A",
			out: &S{
				Code:   "\033[A",
				Type:   "CSI",
				Params: []string{"4", "2"},
			},
			lu:  &CUU_,
			err: ExtraParameters,
		},
		{
			in: "\033[ U",
			out: &S{
				Code:   "\033[ U",
				Type:   "CSI",
				Params: []string{""},
			},
			lu:  &SLH_,
			err: MissingParameters,
		},
		{
			in: "\033[ c",
			out: &S{
				Code:   "\033[ c",
				Type:   "CSI",
				Params: []string{"", "32"},
			},
			lu:  &TCC_,
			err: MissingParameters,
		},
		{
			in: "\033[42 c",
			s:  "\033[42;32 c",
			out: &S{
				Code:   "\033[ c",
				Type:   "CSI",
				Params: []string{"42", "32"},
			},
			lu: &TCC_,
		},
		{
			in: "\033[42;52 c",
			out: &S{
				Code:   "\033[ c",
				Type:   "CSI",
				Params: []string{"42", "52"},
			},
			lu: &TCC_,
		},
		{
			in: "\033[42;52;62 c",
			out: &S{
				Code:   "\033[ c",
				Type:   "CSI",
				Params: []string{"42", "52", "62"},
			},
			lu:  &TCC_,
			err: ExtraParameters,
		},
		{
			in: "\033]string\033\\",
			out: &S{
				Code:   "\033]",
				Type:   "CS",
				Params: []string{"string"},
			},
			lu: &OSC_,
		},
		{
			in:  "\033]string\033\\extra",
			rem: "extra",
			out: &S{
				Code:   "\033]",
				Type:   "CS",
				Params: []string{"string"},
			},
			lu: &OSC_,
		},
		{
			in: "\033]string\033X",
			out: &S{
				Code:   "\033]",
				Type:   "CS",
				Params: []string{"string"},
			},
			err: FoundSOS,
			lu:  &OSC_,
		},
		{
			in:  "\033]string\033Xextra",
			rem: "extra",
			out: &S{
				Code:   "\033]",
				Type:   "CS",
				Params: []string{"string"},
			},
			err: FoundSOS,
			lu:  &OSC_,
		},
		{
			in: "\033]string",
			out: &S{
				Code:   "\033]",
				Type:   "CS",
				Params: []string{"string"},
			},
			err: NoST,
			lu:  &OSC_,
		},
		{
			in: "\033",
			out: &S{
				Code: "\033",
				Type: "C0",
			},
			err: LoneEscape,
			lu:  &ESC_,
		},
		{
			in: "\033\020",
			out: &S{
				Code: "\033\020",
				Type: "ESC",
			},
			err: UnknownEscape,
		},
		{
			in: "\033[?25l",
			out: &S{
				Code:   "\033[l",
				Type:   "CSI",
				Params: []string{"?25"},
			},
			lu: &RM_,
		},
		{
			in:  "\033Nabc",
			rem: "bc",
			out: &S{
				Code:   "\033N",
				Type:   "C1",
				Params: []string{"a"},
			},
			lu: &SS2_,
		},
		{
			in:  "\033Oabc",
			rem: "bc",
			out: &S{
				Code:   "\033O",
				Type:   "C1",
				Params: []string{"a"},
			},
			lu: &SS3_,
		},
		{
			in: "\033O",
			out: &S{
				Code: "\033O",
				Type: "C1",
			},
			lu: &SS3_,
		},
	} {
		remb, out, err := Decode([]byte(tt.in))
		rem := string(remb)
		if rem != tt.rem {
			t.Errorf("%q: got rem %q, want %q", tt.in, rem, tt.rem)
		}
		if err != tt.err {
			t.Errorf("%q: got error %v, want %v", tt.in, err, tt.err)
		}
		want := tt.out
		if tt.err != nil {
			wantCopy := *tt.out
			wantCopy.Error = tt.err
			want = &wantCopy
		}
		if !reflect.DeepEqual(out, want) {
			t.Errorf("%q: got/want\n%+v\n%+v", tt.in, out, want)
		}
		lu := Table[out.Code]
		if lu != tt.lu {
			t.Errorf("%q: got lu %#v, want %#v", tt.in, lu, tt.lu)
		}
		if tt.s == "" {
			tt.s = strings.TrimSuffix(tt.in, tt.rem)
		}
		if err == nil {
			if s := out.String(); s != tt.s {
				t.Errorf("%q: String got %q, want %q", tt.in, s, tt.s)
			}
		}
	}
}

func TestDecoderC0(t *testing.T) {
	zero := Decoder{}
	c0 := Decoder{C0: true}

	for _, tt := range []struct {
		in  string
		rem string
		lu  *Sequence
		out *S
		s   string
		err error
	}{
		{
			in: "abc\r\n",
			out: &S{
				Code: "abc\r\n",
			},
		},
		{
			in: "\tabc",
			out: &S{
				Code: "\tabc",
			},
		},
	} {
		rem, out, err := zero.Decode([]byte(tt.in))
		if string(rem) != tt.rem {
			t.Errorf("zero %q: got rem %q, want %q", tt.in, rem, tt.rem)
		}
		if err != tt.err {
			t.Errorf("zero %q: got error %v, want %v", tt.in, err, tt.err)
		}
		if !reflect.DeepEqual(out, tt.out) {
			t.Errorf("zero %q: got/want\n%+v\n%+v", tt.in, out, tt.out)
		}
	}

	for _, tt := range []struct {
		in  string
		rem string
		lu  *Sequence
		out *S
		s   string
		err error
	}{
		{
			in: "abc\r\n",
			out: &S{
				Code: "abc",
			},
			rem: "\r\n",
		},
		{
			in: "\r",
			out: &S{
				Code: "\r",
				Type: "C0",
			},
			lu: &CR_,
			s:  "\r",
		},
		{
			in: "\n",
			out: &S{
				Code: "\n",
				Type: "C0",
			},
			lu: &LF_,
			s:  "\n",
		},
		{
			in: "\t",
			out: &S{
				Code: "\t",
				Type: "C0",
			},
			lu: &HT_,
			s:  "\t",
		},
		{
			in: "\ta",
			out: &S{
				Code: "\t",
				Type: "C0",
			},
			lu:  &HT_,
			rem: "a",
			s:   "\t",
		},
		{
			in: "a\tb",
			out: &S{
				Code: "a",
			},
			rem: "\tb",
		},
		{
			in: "\007",
			out: &S{
				Code: "\007",
				Type: "C0",
			},
			lu: &BEL_,
			s:  "\007",
		},
		{
			in: "abc\033[A",
			out: &S{
				Code: "abc",
			},
			rem: "\033[A",
		},
	} {
		rem, out, err := c0.Decode([]byte(tt.in))
		if string(rem) != tt.rem {
			t.Errorf("C0 %q: got rem %q, want %q", tt.in, rem, tt.rem)
		}
		if err != tt.err {
			t.Errorf("C0 %q: got error %v, want %v", tt.in, err, tt.err)
		}
		want := tt.out
		if tt.err != nil {
			wantCopy := *tt.out
			wantCopy.Error = tt.err
			want = &wantCopy
		}
		if !reflect.DeepEqual(out, want) {
			t.Errorf("C0 %q: got/want\n%+v\n%+v", tt.in, out, want)
		}
		if tt.lu != nil {
			if lu := Table[out.Code]; lu != tt.lu {
				t.Errorf("C0 %q: got lu %#v, want %#v", tt.in, lu, tt.lu)
			}
		}
		if tt.s == "" && tt.err == nil {
			tt.s = strings.TrimSuffix(tt.in, tt.rem)
		}
		if err == nil && tt.s != "" {
			if got := out.String(); got != tt.s {
				t.Errorf("C0 %q: String got %q, want %q", tt.in, got, tt.s)
			}
		}
	}

	all := c0.DecodeAll([]byte("a\nb"))
	if len(all) != 3 {
		t.Fatalf("DecodeAll: got %d sequences, want 3: %+v", len(all), all)
	}
	if all[0].Code != "a" || all[0].Type != "" {
		t.Errorf("DecodeAll[0]: got %+v, want plain text %q", all[0], "a")
	}
	if all[1].Code != LF || all[1].Type != "C0" {
		t.Errorf("DecodeAll[1]: got %+v, want LF C0", all[1])
	}
	if all[2].Code != "b" || all[2].Type != "" {
		t.Errorf("DecodeAll[2]: got %+v, want plain text %q", all[2], "b")
	}
}

func TestDecoderUTF8(t *testing.T) {
	zero := Decoder{}
	utf8dec := Decoder{UTF8: true}
	shi := "世"
	earth := "🌍"
	cafe := "café"

	rem, out, err := zero.Decode([]byte("hello" + shi))
	if err != nil {
		t.Fatalf("zero hello世: %v", err)
	}
	if string(out.Code) != "hello\xe4\xb8" || string(rem) != "\x96" {
		t.Errorf("zero hello世: got code %q rem %q", out.Code, rem)
	}

	for _, tt := range []struct {
		d    Decoder
		in   string
		rem  string
		lu   *Sequence
		out  *S
		s    string
		err  error
		name string
	}{
		{
			name: "ascii",
			d:    utf8dec,
			in:   "hello",
			out:  &S{Code: "hello"},
		},
		{
			name: "cjk",
			d:    utf8dec,
			in:   "hello" + shi,
			out:  &S{Code: Name("hello" + shi)},
			s:    "hello" + shi,
		},
		{
			name: "emoji",
			d:    utf8dec,
			in:   earth,
			out:  &S{Code: Name(earth)},
			s:    earth,
		},
		{
			name: "latin1 supplement",
			d:    utf8dec,
			in:   cafe,
			out:  &S{Code: Name(cafe)},
			s:    cafe,
		},
		{
			name: "orphan c1 byte",
			d:    utf8dec,
			in:   "\x82",
			out:  &S{Code: "\033B", Type: "C1"},
			lu:   &BPH_,
			s:    "\033B",
		},
		{
			name: "text then orphan c1",
			d:    utf8dec,
			in:   "abc\x82",
			out:  &S{Code: "abc"},
			rem:  "\x82",
		},
		{
			name: "utf8 then escape",
			d:    utf8dec,
			in:   shi + "\033[A",
			out:  &S{Code: Name(shi)},
			rem:  "\033[A",
			s:    shi,
		},
		{
			name: "utf8 and c0",
			d:    Decoder{UTF8: true, C0: true},
			in:   "a\n" + shi,
			out:  &S{Code: "a"},
			rem:  "\n" + shi,
		},
	} {
		rem, out, err := tt.d.Decode([]byte(tt.in))
		if string(rem) != tt.rem {
			t.Errorf("%s %q: got rem %q, want %q", tt.name, tt.in, rem, tt.rem)
		}
		if err != tt.err {
			t.Errorf("%s %q: got error %v, want %v", tt.name, tt.in, err, tt.err)
		}
		want := tt.out
		if tt.err != nil {
			wantCopy := *tt.out
			wantCopy.Error = tt.err
			want = &wantCopy
		}
		if !reflect.DeepEqual(out, want) {
			t.Errorf("%s %q: got/want\n%+v\n%+v", tt.name, tt.in, out, want)
		}
		if tt.lu != nil {
			if lu := Table[out.Code]; lu != tt.lu {
				t.Errorf("%s %q: got lu %#v, want %#v", tt.name, tt.in, lu, tt.lu)
			}
		}
		if tt.s == "" && tt.err == nil {
			tt.s = strings.TrimSuffix(tt.in, tt.rem)
		}
		if err == nil && tt.s != "" {
			if got := out.String(); got != tt.s {
				t.Errorf("%s %q: String got %q, want %q", tt.name, tt.in, got, tt.s)
			}
		}
	}

	all := utf8dec.DecodeAll([]byte("hello\033[A" + shi))
	if len(all) != 3 {
		t.Fatalf("DecodeAll: got %d sequences, want 3: %+v", len(all), all)
	}
	if all[0].Code != "hello" || all[0].Type != "" {
		t.Errorf("DecodeAll[0]: got %+v, want plain text %q", all[0], "hello")
	}
	if all[1].Code != CUU || all[1].Type != "CSI" {
		t.Errorf("DecodeAll[1]: got %+v, want CUU CSI", all[1])
	}
	if all[2].Code != Name(shi) || all[2].Type != "" {
		t.Errorf("DecodeAll[2]: got %+v, want plain text %q", all[2], shi)
	}
}

func TestSFormat(t *testing.T) {
	for _, tt := range []struct {
		s    *S
		fmt  string
		want string
	}{
		{
			s:    &S{Code: "abc"},
			fmt:  "%+v",
			want: `{Code: "abc"}`,
		},
		{
			s:    &S{Code: CUU, Type: "CSI", Params: []string{"42"}},
			fmt:  "%+v",
			want: `{Code: "CUU", Type: "CSI", Params: "42"}`,
		},
		{
			s:    &S{Code: CUU, Type: "CSI", Params: []string{"4", "2"}, Error: ExtraParameters},
			fmt:  "%+v",
			want: `{Code: "CUU", Type: "CSI", Params: "4", "2", Error: too many parameters for function}`,
		},
		{
			s:    &S{Code: OSC, Type: "CS", Params: []string{"string"}, Error: NoST},
			fmt:  "%+v",
			want: `{Code: "OSC", Type: "CS", Params: "string", Error: control string missing string terminator}`,
		},
		{
			s:    &S{Code: Name("\033\x10"), Type: "ESC", Error: UnknownEscape},
			fmt:  "%+v",
			want: `{Code: "\x1b\x10", Type: "ESC", Error: unknown escape sequence}`,
		},
		{
			s:    &S{Code: CUU, Type: "CSI", Params: []string{"42"}},
			fmt:  "%v",
			want: "\033[42A",
		},
		{
			s:    &S{Code: "hello"},
			fmt:  "%s",
			want: "hello",
		},
	} {
		got := fmt.Sprintf(tt.fmt, tt.s)
		if got != tt.want {
			t.Errorf("fmt.Sprintf(%q, %+v): got %q, want %q", tt.fmt, tt.s, got, tt.want)
		}
	}
}

/*
var (
	LoneEscape    = errors.New("escape at end of input")
	UnknownEscape = errors.New("unknown escape sequence")
	NoST          = errors.New("control string missing string terminator")
	FoundSOS      = errors.New("start of string encountered in control string")
	IncompleteCSI = errors.New("incomplete control sequence")
)
*/
