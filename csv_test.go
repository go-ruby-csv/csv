// Copyright (c) the go-ruby-csv/csv authors
//
// SPDX-License-Identifier: BSD-3-Clause

package csv

import (
	"reflect"
	"testing"
)

// eqRow compares two parsed rows for deep equality.
func eqRow(a, b []any) bool { return reflect.DeepEqual(a, b) }

func TestParseLineBasics(t *testing.T) {
	cases := []struct {
		in   string
		want []any
		nilR bool // want a nil row
	}{
		{in: "a,b,c", want: []any{"a", "b", "c"}},
		{in: `a,"b,c",d`, want: []any{"a", "b,c", "d"}},
		{in: `a,"b""c",d`, want: []any{"a", `b"c`, "d"}},
		{in: "a,,c", want: []any{"a", nil, "c"}},
		{in: ",", want: []any{nil, nil}},
		{in: "a", want: []any{"a"}},
		{in: "a,b\n", want: []any{"a", "b"}},
		{in: "", nilR: true},
		{in: "\n", want: []any{}},
		{in: `"a"`, want: []any{"a"}},
		{in: `""`, want: []any{""}},
		{in: `a,"x` + "\n" + `y",c`, want: []any{"a", "x\ny", "c"}},
	}
	for _, c := range cases {
		got, err := ParseLine(c.in, Options{})
		if err != nil {
			t.Fatalf("ParseLine(%q): %v", c.in, err)
		}
		if c.nilR {
			if got != nil {
				t.Errorf("ParseLine(%q) = %#v, want nil", c.in, got)
			}
			continue
		}
		if !eqRow(got, c.want) {
			t.Errorf("ParseLine(%q) = %#v, want %#v", c.in, got, c.want)
		}
	}
}

func TestParseMultilineAndBlanks(t *testing.T) {
	got, err := ParseRows("a,b\r\nc,d\r\n", Options{})
	if err != nil {
		t.Fatal(err)
	}
	want := [][]any{{"a", "b"}, {"c", "d"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("crlf = %#v, want %#v", got, want)
	}

	got, _ = ParseRows("a,b\n\nc,d", Options{})
	want = [][]any{{"a", "b"}, {}, {"c", "d"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("blank = %#v, want %#v", got, want)
	}

	got, _ = ParseRows("a,b\n\nc,d", Options{SkipBlanks: true})
	want = [][]any{{"a", "b"}, {"c", "d"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("skip_blanks = %#v, want %#v", got, want)
	}

	got, _ = ParseRows("", Options{})
	if len(got) != 0 {
		t.Errorf("empty doc = %#v", got)
	}
}

func TestRowSepDetection(t *testing.T) {
	for _, in := range []string{"a,b\r\nc,d", "a,b\rc,d", "a,b\nc,d"} {
		got, err := ParseRows(in, Options{})
		if err != nil {
			t.Fatalf("%q: %v", in, err)
		}
		want := [][]any{{"a", "b"}, {"c", "d"}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("auto %q = %#v", in, got)
		}
	}
	// Quoted newline must not be mistaken for the auto row_sep.
	got, _ := ParseRows("\"a\nb\",c\nd,e", Options{})
	want := [][]any{{"a\nb", "c"}, {"d", "e"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("auto quoted = %#v", got)
	}
	// Explicit row_sep wins.
	got, _ = ParseRows("a,b;c,d", Options{RowSep: ";"})
	want = [][]any{{"a", "b"}, {"c", "d"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("explicit rowsep = %#v", got)
	}
	// A multi-byte row_sep embedded inside a quoted field is preserved verbatim
	// and counts as one line for error reporting.
	got, _ = ParseRows("\"a\r\nb\",c\r\nd,e", Options{RowSep: "\r\n"})
	want = [][]any{{"a\r\nb", "c"}, {"d", "e"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("crlf in quote = %#v", got)
	}
}

func TestDialectOptions(t *testing.T) {
	got, _ := ParseLine("a::b::c", Options{ColSep: "::"})
	if !eqRow(got, []any{"a", "b", "c"}) {
		t.Errorf("multichar col_sep = %#v", got)
	}
	got, _ = ParseLine("a\tb", Options{ColSep: "\t"})
	if !eqRow(got, []any{"a", "b"}) {
		t.Errorf("tab col_sep = %#v", got)
	}
	got, _ = ParseLine(`'a','b,c'`, Options{QuoteChar: "'"})
	if !eqRow(got, []any{"a", "b,c"}) {
		t.Errorf("alt quote = %#v", got)
	}
	got, _ = ParseLine(`"a",b`, Options{NoQuote: true})
	if !eqRow(got, []any{`"a"`, "b"}) {
		t.Errorf("no_quote = %#v", got)
	}
}

func TestStripAndNilEmpty(t *testing.T) {
	got, _ := ParseLine("  a  ,  b  ", Options{StripSpace: true})
	if !eqRow(got, []any{"a", "b"}) {
		t.Errorf("strip space = %#v", got)
	}
	got, _ = ParseLine(`"  a  ",b`, Options{StripSpace: true})
	if !eqRow(got, []any{"  a  ", "b"}) {
		t.Errorf("strip skips quoted = %#v", got)
	}
	got, _ = ParseLine("xxaxx,xxbxx", Options{StripChars: "x"})
	if !eqRow(got, []any{"a", "b"}) {
		t.Errorf("strip chars = %#v", got)
	}
	got, _ = ParseLine("a,,c", Options{NilValueSet: true, NilValue: "NULL"})
	if !eqRow(got, []any{"a", "NULL", "c"}) {
		t.Errorf("nil_value = %#v", got)
	}
	got, _ = ParseLine("a,,c", Options{NilValueSet: true, NilValue: nil})
	if !eqRow(got, []any{"a", nil, "c"}) {
		t.Errorf("nil_value nil = %#v", got)
	}
	got, _ = ParseLine(`a,"",c`, Options{EmptyValueSet: true, EmptyValue: "EMPTY"})
	if !eqRow(got, []any{"a", "EMPTY", "c"}) {
		t.Errorf("empty_value = %#v", got)
	}
	got, _ = ParseLine(`a,"",c`, Options{EmptyValueSet: true, EmptyValue: nil})
	if !eqRow(got, []any{"a", nil, "c"}) {
		t.Errorf("empty_value nil = %#v", got)
	}
}

func TestSkipLines(t *testing.T) {
	got, _ := ParseRows("# c\na,b\n#x\nc,d", Options{SkipLines: `\A#`})
	want := [][]any{{"a", "b"}, {"c", "d"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("skip_lines = %#v", got)
	}
	// ParseLine with skip_lines returns first non-skipped record.
	row, _ := ParseLine("#skip\na,b", Options{SkipLines: `\A#`})
	if !eqRow(row, []any{"a", "b"}) {
		t.Errorf("parse_line skip = %#v", row)
	}
}

func TestLiberalParsing(t *testing.T) {
	got, _ := ParseLine(`a,b"c,d`, Options{LiberalParsing: true})
	if !eqRow(got, []any{"a", `b"c`, "d"}) {
		t.Errorf("liberal mid = %#v", got)
	}
	got, _ = ParseLine(`a,"b"x,d`, Options{LiberalParsing: true})
	if !eqRow(got, []any{"a", `"b"x`, "d"}) {
		t.Errorf("liberal after = %#v", got)
	}
	got, _ = ParseLine(`a"b`, Options{LiberalParsing: true})
	if !eqRow(got, []any{`a"b`}) {
		t.Errorf("liberal start-mid = %#v", got)
	}
}

func TestDoubledQuoteEdges(t *testing.T) {
	cases := []struct {
		in   string
		want []any
		err  bool
	}{
		{in: `"a""`, err: true},
		{in: `"a"""`, want: []any{`a"`}},
		{in: `""`, want: []any{""}},
		{in: `""""`, want: []any{`"`}},
	}
	for _, c := range cases {
		got, err := ParseLine(c.in, Options{})
		if c.err {
			if err == nil {
				t.Errorf("ParseLine(%q) expected error", c.in)
			}
			continue
		}
		if err != nil {
			t.Fatalf("ParseLine(%q): %v", c.in, err)
		}
		if !eqRow(got, c.want) {
			t.Errorf("ParseLine(%q) = %#v, want %#v", c.in, got, c.want)
		}
	}
}

func TestMalformed(t *testing.T) {
	cases := []struct {
		in   string
		msg  string
		line int
	}{
		{in: `a,"b`, msg: "Unclosed quoted field", line: 1},
		{in: "a,\"b\nc,d", msg: "Unclosed quoted field", line: 1},
		{in: `a,b"c,d`, msg: "Illegal quoting", line: 1},
		{in: `a,"b"x,d`, msg: "Any value after quoted field isn't allowed", line: 1},
		{in: "a,b\nc,\"d\ne", msg: "Unclosed quoted field", line: 2},
		{in: "a,b\nc,d\"e", msg: "Illegal quoting", line: 2},
		{in: "r1,x\nr2,y\nr3,\"open\nmore", msg: "Unclosed quoted field", line: 3},
	}
	for _, c := range cases {
		_, err := Parse(c.in, Options{})
		me, ok := err.(*MalformedCSVError)
		if !ok {
			t.Fatalf("Parse(%q) err = %v, want *MalformedCSVError", c.in, err)
		}
		if me.Reason != c.msg || me.Line != c.line {
			t.Errorf("Parse(%q) = %q, want %q in line %d", c.in, me.Error(), c.msg, c.line)
		}
	}
	// ParseLine and ParseRows surface the same error.
	if _, err := ParseLine(`"a`, Options{}); err == nil {
		t.Error("ParseLine unclosed: want error")
	}
	if _, err := ParseRows(`"a`, Options{}); err == nil {
		t.Error("ParseRows unclosed: want error")
	}
}
