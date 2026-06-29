// Copyright (c) the go-ruby-csv/csv authors
//
// SPDX-License-Identifier: BSD-3-Clause

package csv

import (
	"reflect"
	"testing"
)

func TestFieldConverters(t *testing.T) {
	cases := []struct {
		in   string
		conv []string
		want []any
	}{
		{in: "1,2.5,foo", conv: []string{"integer"}, want: []any{1, "2.5", "foo"}},
		{in: "1,2.5,foo", conv: []string{"float"}, want: []any{1.0, 2.5, "foo"}},
		{in: "1,2.5,foo", conv: []string{"numeric"}, want: []any{1, 2.5, "foo"}},
		{in: "42,3.14,abc", conv: []string{"all"}, want: []any{42, 3.14, "abc"}},
		{in: "007,12abc", conv: []string{"integer"}, want: []any{7, "12abc"}},
		{in: " 42 ,x", conv: []string{"integer"}, want: []any{42, "x"}},
		{in: "0x10,x", conv: []string{"integer"}, want: []any{16, "x"}},
		{in: "1_000,x", conv: []string{"integer"}, want: []any{1000, "x"}},
		{in: "+5,-3", conv: []string{"integer"}, want: []any{5, -3}},
		{in: "0b101,0o17", conv: []string{"integer"}, want: []any{5, 15}},
		{in: "1e3,.5,5.", conv: []string{"float"}, want: []any{1000.0, 0.5, 5.0}},
		{in: "1,,3", conv: []string{"integer"}, want: []any{1, nil, 3}}, // nil unconverted
		{in: "inf,nan", conv: []string{"float"}, want: []any{"inf", "nan"}},
		{in: "x", conv: nil, want: []any{"x"}}, // no converters
		{in: "5", conv: []string{"unknown"}, want: []any{"5"}},
	}
	for _, c := range cases {
		got, err := ParseLine(c.in, Options{Converters: c.conv})
		if err != nil {
			t.Fatalf("ParseLine(%q): %v", c.in, err)
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("ParseLine(%q, %v) = %#v, want %#v", c.in, c.conv, got, c.want)
		}
	}
}

func TestDateConverters(t *testing.T) {
	got, _ := ParseLine("2020-01-15,foo", Options{Converters: []string{"date"}})
	d, ok := got[0].(Date)
	if !ok || d.Year != 2020 || d.Month != 1 || d.Day != 15 {
		t.Errorf("date = %#v", got[0])
	}
	if got[1] != "foo" {
		t.Errorf("date trailing = %#v", got[1])
	}

	got, _ = ParseLine("notadate,2020-13-99", Options{Converters: []string{"date"}})
	if got[0] != "notadate" {
		t.Errorf("non-date kept = %#v", got[0])
	}
	// "2020-13-99" matches the regex shape; parseDate fills fields verbatim.
	if _, ok := got[1].(Date); !ok {
		t.Errorf("matched-shape date = %#v", got[1])
	}

	got, _ = ParseLine("2020-01-15T10:30:00,foo", Options{Converters: []string{"date_time"}})
	if dt, ok := got[0].(DateTime); !ok || dt.Time {
		t.Errorf("date_time = %#v", got[0])
	}

	got, _ = ParseLine("2020-01-15T10:30:00", Options{Converters: []string{"time"}})
	if dt, ok := got[0].(DateTime); !ok || !dt.Time {
		t.Errorf("time = %#v", got[0])
	}
	got, _ = ParseLine("nope", Options{Converters: []string{"date_time"}})
	if got[0] != "nope" {
		t.Errorf("non-datetime = %#v", got[0])
	}
}

func TestRubyInteger(t *testing.T) {
	ok := map[string]int{
		"0": 0, "  10  ": 10, "0x1F": 31, "0b11": 3, "0o17": 15, "017": 15,
		"1_000": 1000, "+5": 5, "-5": -5, "0d42": 42, "-0x10": -16,
	}
	for s, want := range ok {
		if v, o := rubyInteger(s); !o || v != want {
			t.Errorf("rubyInteger(%q) = %d,%v want %d", s, v, o, want)
		}
	}
	bad := []string{"12abc", "", "  ", "1.5", "0xG", "1__0", "_5", "5_", "0x", "+", "0b", "_"}
	for _, s := range bad {
		if _, o := rubyInteger(s); o {
			t.Errorf("rubyInteger(%q) accepted", s)
		}
	}
}

func TestRubyFloat(t *testing.T) {
	ok := map[string]float64{
		"1.5": 1.5, "  2.0  ": 2.0, "1e3": 1000.0, ".5": 0.5, "5.": 5.0,
		"1_000.5": 1000.5, "12": 12.0, "1.": 1.0, "+.5": 0.5, "-3.14": -3.14,
		"0x1.8p3": 12.0, "1E2": 100.0,
	}
	for s, want := range ok {
		if v, o := rubyFloat(s); !o || v != want {
			t.Errorf("rubyFloat(%q) = %v,%v want %v", s, v, o, want)
		}
	}
	bad := []string{"inf", "nan", "infinity", "0x1.8p3p", "1.2.3", "abc", "5_", "", "  ", "+", ".", "1e", "e5", "1.2e3e4", "0x"}
	for _, s := range bad {
		if _, o := rubyFloat(s); o {
			t.Errorf("rubyFloat(%q) accepted", s)
		}
	}
}

func TestHeaderConverters(t *testing.T) {
	cases := []struct {
		in   any
		conv []string
		want any
	}{
		{in: "ABC", conv: []string{"downcase"}, want: "abc"},
		{in: "First Name", conv: []string{"symbol"}, want: Symbol("first_name")},
		{in: "Last-Name! (x)", conv: []string{"symbol"}, want: Symbol("lastname_x")},
		{in: "  Age  ", conv: []string{"symbol"}, want: Symbol("age")},
		{in: "Raw Name", conv: []string{"symbol_raw"}, want: Symbol("Raw Name")},
		{in: "X", conv: []string{"unknown"}, want: "X"},
		{in: 42, conv: []string{"downcase"}, want: 42}, // non-string passthrough
		{in: "X", conv: nil, want: "X"},
	}
	for _, c := range cases {
		got := applyHeaderConverters(c.in, c.conv)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("applyHeaderConverters(%v,%v) = %#v want %#v", c.in, c.conv, got, c.want)
		}
	}
}
