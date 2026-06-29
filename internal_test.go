// Copyright (c) the go-ruby-csv/csv authors
//
// SPDX-License-Identifier: BSD-3-Clause

package csv

import "testing"

// TestErrorMessage exercises MalformedCSVError.Error directly.
func TestErrorMessage(t *testing.T) {
	e := &MalformedCSVError{Reason: "Unclosed quoted field", Line: 3}
	if got := e.Error(); got != "Unclosed quoted field in line 3." {
		t.Errorf("Error() = %q", got)
	}
}

// TestHasHeadersVariants covers every branch of hasHeaders, including the
// false-bool and unrecognised-type defaults.
func TestHasHeadersVariants(t *testing.T) {
	cases := []struct {
		h    any
		want bool
	}{
		{h: nil, want: false},
		{h: true, want: true},
		{h: false, want: false},
		{h: "first_row", want: true},
		{h: "true", want: true},
		{h: "other", want: false},
		{h: []string{"a"}, want: true},
		{h: 42, want: false}, // unrecognised type → no headers
	}
	for _, c := range cases {
		if got := hasHeaders(Options{Headers: c.h}); got != c.want {
			t.Errorf("hasHeaders(%#v) = %v, want %v", c.h, got, c.want)
		}
	}
}

// TestStartsWithAtEmpty covers the empty-substring guard.
func TestStartsWithAt(t *testing.T) {
	p := &parser{data: "abc"}
	if p.startsWithAt(0, "") {
		t.Error("empty sub should not match")
	}
	if !p.startsWithAt(0, "ab") {
		t.Error("ab should match at 0")
	}
	if p.startsWithAt(2, "bc") {
		t.Error("bc should not match at 2 (out of range)")
	}
}

// TestExpSignFloat exercises the exponent-sign branch of looksLikeRubyFloat.
func TestExpSignFloat(t *testing.T) {
	for _, s := range []string{"1e+3", "1e-3", "1.5E+2"} {
		if _, ok := rubyFloat(s); !ok {
			t.Errorf("rubyFloat(%q) should accept", s)
		}
	}
}

// TestStripUnderscoresEmpty covers the empty-input fast path directly.
func TestStripUnderscoresEmpty(t *testing.T) {
	if s, ok := stripUnderscores(""); !ok || s != "" {
		t.Errorf("stripUnderscores(\"\") = %q,%v", s, ok)
	}
}

// TestLooksLikeFloatEmpty covers the empty guard directly.
func TestLooksLikeFloatEmpty(t *testing.T) {
	if looksLikeRubyFloat("") {
		t.Error("empty should not look like a float")
	}
}

// TestBareNewlineInQuoted confirms that — like MRI — a bare '\n' inside a quoted
// field is *not* counted as a line boundary when row_sep is multi-byte; only the
// configured row_sep advances the line counter. The unclosed field on the second
// physical record is reported as line 2 (not 3).
func TestBareNewlineInQuoted(t *testing.T) {
	_, err := Parse("\"x\ny\",a\r\nb,\"open", Options{RowSep: "\r\n"})
	me, ok := err.(*MalformedCSVError)
	if !ok {
		t.Fatalf("err = %v, want *MalformedCSVError", err)
	}
	if me.Reason != "Unclosed quoted field" || me.Line != 2 {
		t.Errorf("bare-newline line count = %q, want line 2", me.Error())
	}
}

// TestConsumeUntilSepNewline covers liberal-parsing trailing content that spans
// a newline (the newline-counting branch of consumeUntilSep).
func TestConsumeUntilSepNewline(t *testing.T) {
	// A quoted field, then trailing junk containing a newline, under liberal
	// parsing: the junk up to the next col/row sep is kept verbatim.
	got, err := ParseRows("\"a\"b c,d", Options{LiberalParsing: true})
	if err != nil {
		t.Fatal(err)
	}
	if got[0][0] != `"a"b c` {
		t.Errorf("liberal trailing = %#v", got[0][0])
	}
	// liberal trailing then EOF (no sep) exercises the loop's EOF exit.
	got, _ = ParseRows("\"a\"xyz", Options{LiberalParsing: true})
	if got[0][0] != `"a"xyz` {
		t.Errorf("liberal trailing eof = %#v", got[0][0])
	}
	// With a non-newline row_sep, trailing junk may legitimately contain a
	// literal '\n', exercising consumeUntilSep's newline-counting branch.
	got, _ = ParseRows("\"a\"x\ny;d", Options{LiberalParsing: true, RowSep: ";"})
	if got[0][0] != "\"a\"x\ny" {
		t.Errorf("liberal trailing newline = %#v", got[0][0])
	}
}
