// Copyright 2014 Google Inc.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ansi

import "io"

// maxEmptyReads bounds how many consecutive zero-byte, nil-error reads a
// Reader tolerates before giving up, matching the behavior of bufio.Reader.
const maxEmptyReads = 100

// A Reader decodes a stream of escape sequences from an io.Reader.  It buffers
// input as needed so that a sequence split across multiple reads is decoded as
// a single S, something Decode cannot do on its own.  The zero value is not
// usable; construct one with NewReader or Decoder.NewReader.
//
// Reader is a thin streaming layer over Decoder: all decoding, including the
// UTF8 and C0 options, is performed by the embedded Decoder.
type Reader struct {
	r   io.Reader
	d   Decoder
	buf []byte
	eof bool
	err error
}

// NewReader returns a Reader that decodes the escape sequences read from r
// using the default (zero) Decoder.  It is shorthand for Decoder{}.NewReader(r).
func NewReader(r io.Reader) *Reader {
	return Decoder{}.NewReader(r)
}

// NewReader returns a Reader that decodes the escape sequences read from r
// using the options in d (UTF8 and C0).  The Partial option is managed
// internally and its value in d is ignored.
func (d Decoder) NewReader(r io.Reader) *Reader {
	d.Partial = true
	return &Reader{r: r, d: d}
}

// Next returns the next escape sequence (or run of plain text) from the
// stream.  It returns io.EOF, with a nil *S, once the stream is exhausted.
// A sequence that is truncated by the end of the stream is returned as its
// best-effort S along with the corresponding error (LoneEscape, NoST, or
// IncompleteCSI); trailing bytes that are not a sequence at all (such as an
// incomplete UTF-8 rune) are returned as plain text.
func (rd *Reader) Next() (*S, error) {
	for {
		out, s, err := rd.d.Decode(rd.buf)
		if s != nil || err != nil {
			rd.buf = out
			return s, err
		}
		// s == nil && err == nil means either the buffer is empty or it
		// ends in an incomplete sequence.  Either way we need more input.
		if rd.eof {
			return rd.flush()
		}
		rd.fill()
	}
}

// fill appends one nonempty read from the underlying reader to buf.  It sets
// eof (and records the terminating error) when the reader is exhausted.
func (rd *Reader) fill() {
	var tmp [4096]byte
	for i := 0; i < maxEmptyReads; i++ {
		n, err := rd.r.Read(tmp[:])
		rd.buf = append(rd.buf, tmp[:n]...)
		if err != nil {
			rd.eof = true
			rd.err = err
			return
		}
		if n > 0 {
			return
		}
	}
	rd.eof = true
	rd.err = io.ErrNoProgress
}

// flush drains whatever remains in buf after the stream has ended.  A truncated
// sequence is decoded without the Partial option so its real error surfaces;
// bytes that Decode will not consume (e.g. an incomplete UTF-8 tail) are
// emitted as plain text so no input is silently dropped.
func (rd *Reader) flush() (*S, error) {
	if len(rd.buf) == 0 {
		if rd.err == io.EOF {
			return nil, io.EOF
		}
		// Surface a non-EOF read error once, then report EOF thereafter.
		err := rd.err
		rd.err = io.EOF
		if err == nil {
			err = io.EOF
		}
		return nil, err
	}
	fd := rd.d
	fd.Partial = false
	out, s, err := fd.Decode(rd.buf)
	if s == nil {
		// Decode consumed nothing it could turn into a sequence (an
		// incomplete UTF-8 tail).  Hand the bytes back as plain text.
		s = &S{Code: Name(rd.buf)}
		rd.buf = nil
		return s, nil
	}
	rd.buf = out
	return s, err
}
