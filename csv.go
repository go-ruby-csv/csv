// Copyright (c) the go-ruby-csv/csv authors
//
// SPDX-License-Identifier: BSD-3-Clause

// Package csv is a pure-Go (no cgo) reimplementation of Ruby's CSV standard
// library — the parser and generator behind MRI 4.0.5's csv gem (3.3.5). It
// reproduces Ruby's dialect options (col_sep / row_sep / quote_char), RFC-style
// quoting with embedded quotes, separators and newlines inside quoted fields,
// the headers model (CSV::Row / CSV::Table), the built-in field and header
// converters, and CSV::MalformedCSVError with MRI-exact messages and line
// numbers — all without any Ruby runtime.
//
// It is the CSV backend for go-embedded-ruby, but is a standalone, reusable
// module: a sibling of go-ruby-regexp, go-ruby-erb and go-ruby-yaml.
//
// # Value model
//
// A parsed field is one of: nil (an empty, unquoted field — Ruby's nil), a
// string, or — when field converters run — an int, float64, a [Date] or a
// [DateTime]. Headers turn a parsed document into a [Table] of [Row]s offering
// access by header name or by index, mirroring CSV::Table / CSV::Row.
package csv

import "fmt"

// MalformedCSVError mirrors Ruby's CSV::MalformedCSVError. Its message and the
// 1-based line number match MRI byte-for-byte (e.g. "Unclosed quoted field in
// line 2.").
type MalformedCSVError struct {
	// Reason is the human-readable cause, without the trailing " in line N.".
	Reason string
	// Line is the 1-based line number MRI reports for the error.
	Line int
}

// Error renders the message exactly as MRI's CSV::MalformedCSVError#message.
func (e *MalformedCSVError) Error() string {
	return fmt.Sprintf("%s in line %d.", e.Reason, e.Line)
}

// Options holds the subset of Ruby CSV options this library understands. The
// zero value is *not* MRI's default; use [DefaultOptions] (or leave the relevant
// fields blank and let [normalize] fill them) to get Ruby's defaults
// (col_sep ",", row_sep :auto, quote_char '"').
type Options struct {
	// ColSep is Ruby's :col_sep (the field separator). Empty means ",".
	ColSep string
	// RowSep is Ruby's :row_sep (the record separator). Empty means :auto,
	// which detects "\r\n", "\r" or "\n" from the data when parsing and uses
	// "\n" when generating.
	RowSep string
	// QuoteChar is Ruby's :quote_char. Empty means '"'. Set NoQuote to disable
	// quoting entirely (Ruby's quote_char: nil).
	QuoteChar string
	// NoQuote disables quote processing (Ruby quote_char: nil): quotes are then
	// ordinary characters on both parse and generate.
	NoQuote bool

	// Headers selects header handling, mirroring Ruby's :headers:
	//   nil / false      → no headers (plain rows)
	//   true             → first row is the header row
	//   "first_row"      → same as true
	//   []string{...}    → explicit header names
	Headers any
	// ReturnHeaders mirrors :return_headers — when true and Headers is set, the
	// header row is itself emitted as a Row (with HeaderRow true).
	ReturnHeaders bool
	// WriteHeaders mirrors :write_headers — when generating with Headers set,
	// emit the header row first.
	WriteHeaders bool

	// Converters lists built-in field converter names (e.g. "integer",
	// "float", "numeric", "date", "date_time", "time", "all") applied to each
	// parsed field. Unknown names are ignored, matching nothing.
	Converters []string
	// HeaderConverters lists built-in header converter names ("downcase",
	// "symbol", "symbol_raw") applied to each header.
	HeaderConverters []string

	// SkipBlanks mirrors :skip_blanks — drop wholly empty rows when parsing.
	SkipBlanks bool
	// SkipLines mirrors :skip_lines — a Go regexp (RE2) source; lines whose raw
	// text matches are skipped. Empty disables it.
	SkipLines string
	// LiberalParsing mirrors :liberal_parsing — tolerate quotes that would
	// otherwise be malformed, keeping the raw text instead of erroring.
	LiberalParsing bool

	// Strip mirrors :strip. StripSpace strips leading/trailing whitespace from
	// unquoted fields; StripChars (when non-empty) strips those specific
	// characters instead (Ruby strip: "x").
	StripSpace bool
	StripChars string

	// ForceQuotes mirrors :force_quotes — quote every generated field.
	ForceQuotes bool
	// QuoteEmpty mirrors :quote_empty (default true) — quote empty (but
	// non-nil) string fields when generating. Set QuoteEmptySet to make a false
	// value meaningful (otherwise the zero value is treated as the default true).
	QuoteEmpty    bool
	QuoteEmptySet bool

	// NilValue mirrors :nil_value — value substituted for an empty unquoted
	// field on parse (default nil). NilValueSet distinguishes "set to nil"
	// from "unset".
	NilValue    any
	NilValueSet bool
	// EmptyValue mirrors :empty_value — value substituted for an empty *quoted*
	// field on parse (default ""). EmptyValueSet distinguishes "set to nil".
	EmptyValue    any
	EmptyValueSet bool
}

// DefaultOptions returns a copy of opts with Ruby's defaults filled in for any
// blank dialect field, so callers can reason about effective separators.
func DefaultOptions(opts Options) Options {
	n := opts
	n.colSep()
	return n
}

// colSep returns the effective column separator (Ruby default ",").
func (o *Options) colSep() string {
	if o.ColSep == "" {
		return ","
	}
	return o.ColSep
}

// quote returns the effective quote character (Ruby default '"') and whether
// quoting is enabled at all.
func (o *Options) quote() (string, bool) {
	if o.NoQuote {
		return "", false
	}
	if o.QuoteChar == "" {
		return `"`, true
	}
	return o.QuoteChar, true
}

// quoteEmpty reports the effective :quote_empty (default true).
func (o *Options) quoteEmpty() bool {
	if !o.QuoteEmptySet {
		return true
	}
	return o.QuoteEmpty
}
