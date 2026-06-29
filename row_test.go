// Copyright (c) the go-ruby-csv/csv authors
//
// SPDX-License-Identifier: BSD-3-Clause

package csv

import (
	"reflect"
	"testing"
)

func parseTable(t *testing.T, data string, opts Options) *Table {
	t.Helper()
	v, err := Parse(data, opts)
	if err != nil {
		t.Fatalf("Parse(%q): %v", data, err)
	}
	tbl, ok := v.(*Table)
	if !ok {
		t.Fatalf("Parse(%q) = %T, want *Table", data, v)
	}
	return tbl
}

func TestHeadersTrue(t *testing.T) {
	tbl := parseTable(t, "name,age\nAlice,30\nBob,25", Options{Headers: true})
	if !reflect.DeepEqual(tbl.Headers, []any{"name", "age"}) {
		t.Errorf("headers = %#v", tbl.Headers)
	}
	if len(tbl.Rows) != 2 {
		t.Fatalf("rows = %d", len(tbl.Rows))
	}
	if v, _ := tbl.Rows[0].Field("name"); v != "Alice" {
		t.Errorf("by name = %v", v)
	}
	if v, _ := tbl.Rows[0].At(1); v != "30" {
		t.Errorf("by index = %v", v)
	}
	if v, _ := tbl.Rows[1].At(-1); v != "25" {
		t.Errorf("neg index = %v", v)
	}
	want := [][]any{{"name", "age"}, {"Alice", "30"}, {"Bob", "25"}}
	if !reflect.DeepEqual(tbl.ToArray(), want) {
		t.Errorf("to_a = %#v", tbl.ToArray())
	}
	h := tbl.Rows[0].ToHash()
	if h["name"] != "Alice" || h["age"] != "30" {
		t.Errorf("to_h = %#v", h)
	}
}

func TestHeadersFirstRowString(t *testing.T) {
	tbl := parseTable(t, "name,age\nAlice,30", Options{Headers: "first_row"})
	if !reflect.DeepEqual(tbl.Headers, []any{"name", "age"}) {
		t.Errorf("first_row headers = %#v", tbl.Headers)
	}
	tbl = parseTable(t, "name,age\nAlice,30", Options{Headers: "true"})
	if !reflect.DeepEqual(tbl.Headers, []any{"name", "age"}) {
		t.Errorf("\"true\" headers = %#v", tbl.Headers)
	}
}

func TestHeadersArray(t *testing.T) {
	tbl := parseTable(t, "Alice,30\nBob,25", Options{Headers: []string{"name", "age"}})
	if !reflect.DeepEqual(tbl.Headers, []any{"name", "age"}) {
		t.Errorf("array headers = %#v", tbl.Headers)
	}
	if v, _ := tbl.Rows[0].Field("name"); v != "Alice" {
		t.Errorf("array by name = %v", v)
	}
	want := [][]any{{"name", "age"}, {"Alice", "30"}, {"Bob", "25"}}
	if !reflect.DeepEqual(tbl.ToArray(), want) {
		t.Errorf("array to_a = %#v", tbl.ToArray())
	}
}

func TestReturnHeaders(t *testing.T) {
	tbl := parseTable(t, "name,age\nAlice,30", Options{Headers: true, ReturnHeaders: true})
	if len(tbl.Rows) != 2 || !tbl.Rows[0].HeaderRow {
		t.Fatalf("return_headers rows = %#v", tbl.Rows)
	}
	want := [][]any{{"name", "age"}, {"Alice", "30"}}
	if !reflect.DeepEqual(tbl.ToArray(), want) {
		t.Errorf("return_headers to_a = %#v", tbl.ToArray())
	}
}

func TestHeadersWithConverters(t *testing.T) {
	tbl := parseTable(t, "First Name,Age\nAlice,30",
		Options{Headers: true, HeaderConverters: []string{"symbol"}, Converters: []string{"integer"}})
	if !reflect.DeepEqual(tbl.Headers, []any{Symbol("first_name"), Symbol("age")}) {
		t.Errorf("converted headers = %#v", tbl.Headers)
	}
	if v, _ := tbl.Rows[0].Field(Symbol("first_name")); v != "Alice" {
		t.Errorf("by symbol = %v", v)
	}
	if v, _ := tbl.Rows[0].Field(Symbol("age")); v != 30 {
		t.Errorf("converted field = %v", v)
	}
}

func TestHeadersDuplicateAndMissing(t *testing.T) {
	tbl := parseTable(t, "a,a,b\n1,2,3", Options{Headers: true})
	if v, _ := tbl.Rows[0].Field("a"); v != "1" {
		t.Errorf("dup first wins = %v", v)
	}
	if _, ok := tbl.Rows[0].Field("missing"); ok {
		t.Error("missing header should report ok=false")
	}
	// to_h: last value wins for duplicate keys.
	h := tbl.Rows[0].ToHash()
	if h["a"] != "2" {
		t.Errorf("dup to_h last wins = %#v", h)
	}
}

func TestHeadersEmptyDoc(t *testing.T) {
	tbl := parseTable(t, "", Options{Headers: true})
	if len(tbl.Rows) != 0 || tbl.Headers != nil {
		t.Errorf("empty headers doc = %#v", tbl)
	}
}

func TestRowAndTableBounds(t *testing.T) {
	tbl := parseTable(t, "name\nAlice", Options{Headers: true})
	r := tbl.Rows[0]
	if _, ok := r.At(5); ok {
		t.Error("At(5) should be out of range")
	}
	if _, ok := r.At(-5); ok {
		t.Error("At(-5) should be out of range")
	}
	if _, ok := tbl.Row(5); ok {
		t.Error("Row(5) out of range")
	}
	if _, ok := tbl.Row(-5); ok {
		t.Error("Row(-5) out of range")
	}
	if got, ok := tbl.Row(-1); !ok || got != r {
		t.Error("Row(-1) should be last")
	}
	// Short row: Field for a header beyond the fields returns nil,true.
	short := &Row{Headers: []any{"a", "b"}, Fields: []any{"x"}}
	if v, ok := short.Field("b"); !ok || v != nil {
		t.Errorf("short Field = %v,%v", v, ok)
	}
	if h := short.ToHash(); h["b"] != nil {
		t.Errorf("short to_h = %#v", h)
	}
}

func TestParseNoHeadersReturnsRows(t *testing.T) {
	v, err := Parse("a,b\nc,d", Options{})
	if err != nil {
		t.Fatal(err)
	}
	rows, ok := v.([][]any)
	if !ok {
		t.Fatalf("no-header Parse = %T", v)
	}
	want := [][]any{{"a", "b"}, {"c", "d"}}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("rows = %#v", rows)
	}
	// With converters on the no-header path.
	v, _ = Parse("1,2\n3,4", Options{Converters: []string{"integer"}})
	rows = v.([][]any)
	if rows[0][0] != 1 {
		t.Errorf("no-header converters = %#v", rows)
	}
}

func TestDefaultOptions(t *testing.T) {
	o := DefaultOptions(Options{})
	if o.colSep() != "," {
		t.Errorf("default col_sep = %q", o.colSep())
	}
	o = DefaultOptions(Options{ColSep: ";"})
	if o.ColSep != ";" {
		t.Errorf("kept col_sep = %q", o.ColSep)
	}
}
