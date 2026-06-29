// Copyright (c) the go-ruby-csv/csv authors
//
// SPDX-License-Identifier: BSD-3-Clause

package csv

import (
	"fmt"
	"strings"
)

// GenerateLine renders one row to a CSV record (with the row separator),
// mirroring Ruby's CSV.generate_line. A nil field becomes an empty (unquoted)
// string; other fields are stringified with Ruby's to_s rules for the common
// scalar types (the binding pre-stringifies anything exotic).
func GenerateLine(row []any, opts Options) (string, error) {
	return genLine(row, opts), nil
}

// genLine is the infallible core of GenerateLine. The public API keeps an error
// return for symmetry with the parser and for the binding, but generation never
// fails for the supported value types.
func genLine(row []any, opts Options) string {
	colSep := opts.colSep()
	quoteChar, quoting := opts.quote()
	rowSep := opts.RowSep
	if rowSep == "" {
		rowSep = "\n" // generate default $INPUT_RECORD_SEPARATOR
	}

	var b strings.Builder
	for i, f := range row {
		if i > 0 {
			b.WriteString(colSep)
		}
		b.WriteString(renderField(f, colSep, rowSep, quoteChar, quoting, opts))
	}
	b.WriteString(rowSep)
	return b.String()
}

// Generate renders many rows, mirroring Ruby's CSV.generate. When WriteHeaders
// is set with a []string Headers, the header row is emitted first.
func Generate(rows [][]any, opts Options) (string, error) {
	var b strings.Builder
	if opts.WriteHeaders {
		if hs, ok := opts.Headers.([]string); ok {
			hrow := make([]any, len(hs))
			for i, s := range hs {
				hrow[i] = s
			}
			b.WriteString(genLine(hrow, opts))
		}
	}
	for _, r := range rows {
		b.WriteString(genLine(r, opts))
	}
	return b.String(), nil
}

// renderField formats and (if needed) quotes one field, following CSV::Writer's
// rules: a field is quoted when force_quotes is set; or it is an empty string
// and quote_empty is on; or it contains the quote char, the col_sep, the row_sep,
// a CR or an LF. Embedded quote chars are doubled.
func renderField(f any, colSep, rowSep, quoteChar string, quoting bool, opts Options) string {
	if f == nil {
		return "" // Ruby nil → empty, never quoted
	}
	s := toRubyString(f)

	if !quoting {
		return s
	}

	mustQuote := opts.ForceQuotes
	if s == "" {
		if opts.quoteEmpty() {
			mustQuote = true
		}
	} else if !mustQuote {
		if strings.Contains(s, quoteChar) ||
			strings.Contains(s, colSep) ||
			strings.Contains(s, rowSep) ||
			strings.ContainsAny(s, "\r\n") {
			mustQuote = true
		}
	}

	if !mustQuote {
		return s
	}
	escaped := strings.ReplaceAll(s, quoteChar, quoteChar+quoteChar)
	return quoteChar + escaped + quoteChar
}

// toRubyString renders a field value with Ruby's to_s for the scalar types the
// generator handles directly. Strings pass through; numbers use Ruby-compatible
// formatting; everything else uses Go's default, which the binding overrides by
// pre-stringifying.
func toRubyString(f any) string {
	switch v := f.(type) {
	case string:
		return v
	case Symbol:
		return string(v)
	case bool:
		if v {
			return "true"
		}
		return "false"
	case int:
		return fmt.Sprintf("%d", v)
	case int64:
		return fmt.Sprintf("%d", v)
	case float64:
		return formatFloat(v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// formatFloat renders a float64 the way Ruby's Float#to_s does for the values
// CSV round-trips (always at least one fractional digit, e.g. "2.0").
func formatFloat(f float64) string {
	s := fmt.Sprintf("%g", f)
	if !strings.ContainsAny(s, ".eEnN") {
		s += ".0"
	}
	return s
}
