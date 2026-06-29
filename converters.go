// Copyright (c) the go-ruby-csv/csv authors
//
// SPDX-License-Identifier: BSD-3-Clause

package csv

import (
	"regexp"
	"strings"
)

// Date mirrors a Ruby Date produced by the :date converter. It carries the
// calendar date; the binding maps it to a Ruby Date object.
type Date struct {
	Year, Month, Day int
	// Raw is the original field text Date.parse consumed (for the binding to
	// re-parse faithfully if it needs the exact Ruby object).
	Raw string
}

// DateTime mirrors a Ruby DateTime / Time produced by the :date_time / :time
// converters. Time reports whether it came from the :time converter (Ruby Time)
// rather than :date_time (Ruby DateTime).
type DateTime struct {
	Raw  string
	Time bool
}

// Symbol mirrors a Ruby Symbol produced by the :symbol / :symbol_raw header
// converters.
type Symbol string

// dateMatcher / dateTimeMatcher mirror MRI's DateMatcher / DateTimeMatcher
// regexps verbatim (translated to RE2). \w in Ruby is [A-Za-z0-9_]; \s is the
// ASCII whitespace class; both are equivalent in RE2 for ASCII input.
var (
	dateMatcher = regexp.MustCompile(
		`\A(?:(\w+,?\s+)?\w+\s+\d{1,2},?\s+\d{2,4}|\d{4}-\d{2}-\d{2})\z`)
	dateTimeMatcher = regexp.MustCompile(
		`\A(?:(\w+,?\s+)?\w+\s+\d{1,2}\s+\d{1,2}:\d{1,2}:\d{1,2},?\s+\d{2,4}|` +
			`\d{4}-\d{2}-\d{2}(?:[T\s]\d{2}:\d{2}(?::\d{2}(?:\.\d+)?(?:[+-]\d{2}(?::\d{2})|Z)?)?)?)\z`)
)

// applyConverters runs the named field converters in order over a parsed row.
// A nil field is never converted (matching MRI, which skips nil). The first
// converter to change the field's class stops the chain for that field, exactly
// like CSV's convert loop.
func applyConverters(row []any, names []string) {
	if len(names) == 0 {
		return
	}
	procs := expandConverters(names, fieldConverterTable)
	for i, f := range row {
		s, ok := f.(string)
		if !ok {
			continue // nil or already converted
		}
		for _, conv := range procs {
			if v, changed := conv(s); changed {
				row[i] = v
				break
			}
		}
	}
}

// convResult is a single built-in converter: it returns the converted value and
// whether it applied (changed the class).
type convResult func(string) (any, bool)

// expandConverters resolves names (which may reference composite converters like
// "numeric" = [integer,float]) into a flat ordered list of leaf converters.
func expandConverters(names []string, table map[string][]string) []convResult {
	var out []convResult
	var walk func(name string)
	walk = func(name string) {
		if children, ok := table[name]; ok {
			for _, c := range children {
				walk(c)
			}
			return
		}
		if leaf, ok := leafFieldConverters[name]; ok {
			out = append(out, leaf)
		}
		// Unknown names are ignored (match nothing), like an absent key.
	}
	for _, n := range names {
		walk(n)
	}
	return out
}

// fieldConverterTable expands the composite field converter names exactly as
// CSV::Converters does: numeric = [integer, float]; all = [date_time, numeric].
var fieldConverterTable = map[string][]string{
	"numeric": {"integer", "float"},
	"all":     {"date_time", "numeric"},
}

// leafFieldConverters holds the primitive field converters.
var leafFieldConverters = map[string]convResult{
	"integer": func(s string) (any, bool) {
		if v, ok := rubyInteger(s); ok {
			return v, true
		}
		return nil, false
	},
	"float": func(s string) (any, bool) {
		if v, ok := rubyFloat(s); ok {
			return v, true
		}
		return nil, false
	},
	"date": func(s string) (any, bool) {
		if dateMatcher.MatchString(s) {
			if d, ok := parseDate(s); ok {
				return d, true
			}
		}
		return nil, false
	},
	"date_time": func(s string) (any, bool) {
		if dateTimeMatcher.MatchString(s) {
			return DateTime{Raw: s, Time: false}, true
		}
		return nil, false
	},
	"time": func(s string) (any, bool) {
		if dateTimeMatcher.MatchString(s) {
			return DateTime{Raw: s, Time: true}, true
		}
		return nil, false
	},
}

// parseDate extracts year/month/day from an ISO yyyy-mm-dd field (the common
// case the binding can map directly). For the other DateMatcher shapes it keeps
// Raw set so the binding can defer to Ruby's Date.parse.
func parseDate(s string) (Date, bool) {
	d := Date{Raw: s}
	if m := isoDate.FindStringSubmatch(s); m != nil {
		d.Year = atoi(m[1])
		d.Month = atoi(m[2])
		d.Day = atoi(m[3])
	}
	return d, true
}

var isoDate = regexp.MustCompile(`\A(\d{4})-(\d{2})-(\d{2})\z`)

// atoi parses a non-negative decimal string known to be digits.
func atoi(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		n = n*10 + int(s[i]-'0')
	}
	return n
}

// applyHeaderConverters runs the named header converters over a header value
// (always a string at entry), returning the possibly-retyped header (string or
// Symbol).
func applyHeaderConverters(h any, names []string) any {
	s, ok := h.(string)
	if !ok || len(names) == 0 {
		return h
	}
	var cur any = s
	for _, name := range names {
		conv, ok := headerConverters[name]
		if !ok {
			continue
		}
		if str, ok := cur.(string); ok {
			cur = conv(str)
		}
	}
	return cur
}

// headerConverters mirrors CSV::HeaderConverters.
var headerConverters = map[string]func(string) any{
	"downcase": func(h string) any { return strings.ToLower(h) },
	"symbol": func(h string) any {
		low := strings.ToLower(h)
		low = symbolStrip.ReplaceAllString(low, "")
		low = strings.TrimFunc(low, isRubySpace)
		low = symbolSpace.ReplaceAllString(low, "_")
		return Symbol(low)
	},
	"symbol_raw": func(h string) any { return Symbol(h) },
}

var (
	// symbolStrip removes runs of characters that are neither whitespace nor
	// word characters: Ruby /[^\s\w]+/.
	symbolStrip = regexp.MustCompile(`[^\s\w]+`)
	// symbolSpace collapses whitespace runs to a single underscore: /\s+/.
	symbolSpace = regexp.MustCompile(`\s+`)
)

// isRubySpace reports whether r is in Ruby's \s ASCII whitespace class.
func isRubySpace(r rune) bool {
	switch r {
	case ' ', '\t', '\n', '\r', '\f', '\v':
		return true
	}
	return false
}
