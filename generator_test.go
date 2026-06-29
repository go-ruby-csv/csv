// Copyright (c) the go-ruby-csv/csv authors
//
// SPDX-License-Identifier: BSD-3-Clause

package csv

import "testing"

func TestGenerateLine(t *testing.T) {
	cases := []struct {
		row  []any
		opts Options
		want string
	}{
		{row: []any{"a", "b", "c"}, want: "a,b,c\n"},
		{row: []any{"a", "b,c", "d"}, want: "a,\"b,c\",d\n"},
		{row: []any{"a", `b"c`, "d"}, want: "a,\"b\"\"c\",d\n"},
		{row: []any{"a", "x\ny", "c"}, want: "a,\"x\ny\",c\n"},
		{row: []any{1, 2.5, nil, "x"}, want: "1,2.5,,x\n"},
		{row: []any{2.0}, want: "2.0\n"},
		{row: []any{"a\rb"}, want: "\"a\rb\"\n"},
		{row: []any{"a", "b"}, opts: Options{ColSep: "\t"}, want: "a\tb\n"},
		{row: []any{"a", "b"}, opts: Options{RowSep: "\r\n"}, want: "a,b\r\n"},
		{row: []any{"a", "b"}, opts: Options{ForceQuotes: true}, want: "\"a\",\"b\"\n"},
		{row: []any{"", "b"}, opts: Options{QuoteEmptySet: true, QuoteEmpty: true}, want: "\"\",b\n"},
		{row: []any{"", "b"}, opts: Options{QuoteEmptySet: true, QuoteEmpty: false}, want: ",b\n"},
		{row: []any{"", "b"}, want: "\"\",b\n"}, // default quote_empty true
		{row: []any{nil, ""}, want: ",\"\"\n"},
		{row: []any{`"a"`, "b"}, opts: Options{NoQuote: true}, want: "\"a\",b\n"},
		{row: []any{true, false, Symbol("sym")}, want: "true,false,sym\n"},
		{row: []any{int64(7)}, want: "7\n"},
		{row: []any{[]int{1}}, want: "[1]\n"}, // fallback %v
	}
	for _, c := range cases {
		got, err := GenerateLine(c.row, c.opts)
		if err != nil {
			t.Fatalf("GenerateLine(%#v): %v", c.row, err)
		}
		if got != c.want {
			t.Errorf("GenerateLine(%#v) = %q, want %q", c.row, got, c.want)
		}
	}
}

func TestGenerate(t *testing.T) {
	got, err := Generate([][]any{{"a", "b"}, {"c", "d"}}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got != "a,b\nc,d\n" {
		t.Errorf("Generate = %q", got)
	}

	got, _ = Generate([][]any{{"1", "2"}},
		Options{Headers: []string{"a", "b"}, WriteHeaders: true})
	if got != "a,b\n1,2\n" {
		t.Errorf("write_headers = %q", got)
	}

	// write_headers without a []string Headers emits nothing extra.
	got, _ = Generate([][]any{{"1", "2"}}, Options{Headers: true, WriteHeaders: true})
	if got != "1,2\n" {
		t.Errorf("write_headers non-array = %q", got)
	}
}

func TestRoundTrip(t *testing.T) {
	rows := [][]any{
		{"a", "b,c", "d\ne"},
		{"x\"y", "", "z"},
	}
	data, err := Generate(rows, Options{})
	if err != nil {
		t.Fatal(err)
	}
	got, err := ParseRows(data, Options{})
	if err != nil {
		t.Fatal(err)
	}
	// Empty string round-trips as "" (it was quoted on generate).
	want := [][]any{
		{"a", "b,c", "d\ne"},
		{"x\"y", "", "z"},
	}
	for i := range want {
		if !eqRow(got[i], want[i]) {
			t.Errorf("round trip row %d = %#v, want %#v", i, got[i], want[i])
		}
	}
}
