// Copyright (c) the go-ruby-csv/csv authors
//
// SPDX-License-Identifier: BSD-3-Clause

package csv

import (
	"regexp"
	"strings"
)

// ParseLine parses a single CSV record, mirroring Ruby's CSV.parse_line. It
// returns the row's fields ([]any) or nil when the line is empty (Ruby returns
// nil for an empty string). A record may legitimately span several physical
// lines when a field is quoted and contains the row separator; ParseLine reads
// exactly one logical record and ignores any trailing records.
func ParseLine(line string, opts Options) ([]any, error) {
	rows, err := parseAll(line, opts, true)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	applyConverters(rows[0], opts.Converters)
	return rows[0], nil
}

// Parse parses an entire CSV document, mirroring Ruby's CSV.parse. Without
// headers it returns [][]any. With headers set it returns a *Table; callers that
// want the rows can use Table.Rows or the convenience [Parse] result type
// switch. The returned value is `any` because Ruby's CSV.parse returns either an
// Array of Arrays or a CSV::Table depending on :headers.
func Parse(data string, opts Options) (any, error) {
	rows, err := parseAll(data, opts, false)
	if err != nil {
		return nil, err
	}
	if !hasHeaders(opts) {
		out := make([][]any, len(rows))
		copy(out, rows)
		for _, r := range out {
			applyConverters(r, opts.Converters)
		}
		return out, nil
	}
	return buildTable(rows, opts)
}

// ParseRows parses a document with no header handling and returns the raw rows.
// It is the typed entry point the binding uses when it knows headers are off.
func ParseRows(data string, opts Options) ([][]any, error) {
	rows, err := parseAll(data, opts, false)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		applyConverters(r, opts.Converters)
	}
	return rows, nil
}

// hasHeaders reports whether opts requests header handling.
func hasHeaders(opts Options) bool {
	switch h := opts.Headers.(type) {
	case nil:
		return false
	case bool:
		return h
	case string:
		return h == "first_row" || h == "true"
	case []string:
		return true
	default:
		return false
	}
}

// detectRowSep implements Ruby's row_sep: :auto. It scans for the first bare
// (unquoted) CR/LF and returns "\r\n", "\r" or "\n"; absent any, it defaults to
// "\n" (Ruby falls back to $INPUT_RECORD_SEPARATOR, i.e. "\n").
func detectRowSep(data string, quoteChar string, quoting bool) string {
	inQuote := false
	for i := 0; i < len(data); i++ {
		c := data[i]
		if quoting && len(quoteChar) == 1 && c == quoteChar[0] {
			inQuote = !inQuote
			continue
		}
		if inQuote {
			continue
		}
		switch c {
		case '\r':
			if i+1 < len(data) && data[i+1] == '\n' {
				return "\r\n"
			}
			return "\r"
		case '\n':
			return "\n"
		}
	}
	return "\n"
}

// parseAll is the shared engine for ParseLine / Parse. When single is true it
// stops after the first record (CSV.parse_line semantics).
func parseAll(data string, opts Options, single bool) ([][]any, error) {
	colSep := opts.colSep()
	quoteChar, quoting := opts.quote()
	rowSep := opts.RowSep
	if rowSep == "" {
		rowSep = detectRowSep(data, quoteChar, quoting)
	}

	var skipRe *regexp.Regexp
	if opts.SkipLines != "" {
		skipRe = regexp.MustCompile(opts.SkipLines)
	}

	p := &parser{
		data:      data,
		colSep:    colSep,
		rowSep:    rowSep,
		quoteChar: quoteChar,
		quoting:   quoting,
		opts:      opts,
		skipRe:    skipRe,
	}
	return p.run(single)
}

// parser is the byte-level CSV state machine.
type parser struct {
	data      string
	colSep    string
	rowSep    string
	quoteChar string
	quoting   bool
	opts      Options
	skipRe    *regexp.Regexp

	pos  int // current byte offset into data
	line int // 1-based line number of the record currently being read
}

// run scans data into records.
func (p *parser) run(single bool) ([][]any, error) {
	var rows [][]any
	p.line = 1
	for p.pos < len(p.data) {
		row, raw, err := p.readRecord()
		if err != nil {
			return nil, err
		}
		// skip_lines: drop records whose raw source matches.
		if p.skipRe != nil && p.skipRe.MatchString(raw) {
			continue
		}
		// skip_blanks: drop wholly empty records.
		if p.opts.SkipBlanks && len(row) == 0 {
			continue
		}
		rows = append(rows, row)
		if single {
			break
		}
	}
	return rows, nil
}

// readRecord reads one logical record starting at p.pos (which run guarantees is
// < len(data), so at least one byte is consumed). It returns the parsed fields,
// the raw textual line (for skip_lines) and an error. p.line is advanced past
// the record.
func (p *parser) readRecord() (row []any, raw string, err error) {
	startPos := p.pos
	var fields []any
	var field strings.Builder
	fieldStarted := false // whether the current field has any committed bytes
	quotedField := false  // whether the current field began with a quote

	// flush commits the in-progress field as a value.
	flush := func() {
		v := p.makeField(field.String(), fieldStarted, quotedField)
		fields = append(fields, v)
		field.Reset()
		fieldStarted = false
		quotedField = false
	}

	for {
		// End of data: terminate the record (run guarantees we consumed ≥1 byte).
		if p.pos >= len(p.data) {
			flush()
			raw = p.data[startPos:p.pos]
			return p.finishRow(fields), raw, nil
		}

		// A quoted field begins only at the very start of a field.
		if p.quoting && !fieldStarted && !quotedField && p.startsWith(p.quoteChar) {
			val, e := p.readQuoted()
			if e != nil {
				return nil, "", e
			}
			field.WriteString(val)
			fieldStarted = true
			quotedField = true
			continue
		}

		// Row separator → end of record.
		if p.startsWith(p.rowSep) {
			p.pos += len(p.rowSep)
			p.line++
			// An empty physical line (no field began, no field committed) is an
			// empty record [] in MRI, not [nil].
			if fieldStarted || quotedField || len(fields) > 0 {
				flush()
			}
			raw = p.data[startPos : p.pos-len(p.rowSep)]
			return p.finishRow(fields), raw, nil
		}

		// Column separator → end of field.
		if p.startsWith(p.colSep) {
			p.pos += len(p.colSep)
			flush()
			continue
		}

		// A bare quote inside an unquoted field. (A field that *began* with a
		// quote is consumed wholesale by readQuoted, so control only returns
		// here for fields that started unquoted; reaching a quote here is thus a
		// stray quote.)
		if p.quoting && p.startsWith(p.quoteChar) {
			if p.opts.LiberalParsing {
				field.WriteString(p.quoteChar)
				fieldStarted = true
				p.pos += len(p.quoteChar)
				continue
			}
			return nil, "", p.malformed("Illegal quoting")
		}

		// Ordinary byte.
		field.WriteByte(p.data[p.pos])
		fieldStarted = true
		p.pos++
	}
}

// readQuoted reads a quoted field's interior (p.pos is at the opening quote).
// It returns the decoded contents. On EOF before a closing quote it returns a
// MalformedCSVError. After the closing quote it permits only a col_sep, row_sep
// or EOF to follow (else "Any value after quoted field isn't allowed", unless
// liberal_parsing keeps the trailing bytes as part of the field).
func (p *parser) readQuoted() (string, error) {
	openLine := p.line
	p.pos += len(p.quoteChar) // consume opening quote
	var b strings.Builder
	for {
		if p.pos >= len(p.data) {
			return "", &MalformedCSVError{Reason: "Unclosed quoted field", Line: openLine}
		}
		if p.startsWith(p.quoteChar) {
			// Doubled quote → literal quote.
			if p.pos+len(p.quoteChar) < len(p.data) &&
				p.startsWithAt(p.pos+len(p.quoteChar), p.quoteChar) {
				b.WriteString(p.quoteChar)
				p.pos += 2 * len(p.quoteChar)
				continue
			}
			// Closing quote.
			p.pos += len(p.quoteChar)
			// What follows the close?
			if p.pos >= len(p.data) || p.startsWith(p.colSep) || p.startsWith(p.rowSep) {
				return b.String(), nil
			}
			// A lone quote here is the "" escape only when paired — otherwise
			// trailing content after the closed field.
			if p.opts.LiberalParsing {
				// Keep the raw quoted text plus trailing bytes verbatim. Ruby's
				// liberal_parsing preserves the field including its quotes.
				rest := p.consumeUntilSep()
				return p.quoteChar + b.String() + p.quoteChar + rest, nil
			}
			return "", p.malformed("Any value after quoted field isn't allowed")
		}
		// A row separator inside the quoted field is literal content, but it
		// still advances the line counter (matching MRI, which counts only the
		// configured row_sep — never a bare stray newline — within a field).
		if p.startsWith(p.rowSep) {
			b.WriteString(p.rowSep)
			p.pos += len(p.rowSep)
			p.line++
			continue
		}
		b.WriteByte(p.data[p.pos])
		p.pos++
	}
}

// consumeUntilSep reads bytes until the next col_sep, row_sep or EOF, returning
// them (used by liberal_parsing for trailing junk after a quoted field).
func (p *parser) consumeUntilSep() string {
	start := p.pos
	for p.pos < len(p.data) {
		if p.startsWith(p.colSep) || p.startsWith(p.rowSep) {
			break
		}
		if p.data[p.pos] == '\n' {
			p.line++
		}
		p.pos++
	}
	return p.data[start:p.pos]
}

// finishRow applies strip and produces the final field slice for a record.
func (p *parser) finishRow(fields []any) []any {
	if len(fields) == 0 {
		return []any{}
	}
	return fields
}

// makeField turns the accumulated bytes of one field into its value, applying
// nil_value / empty_value and strip. quoted reports whether the field was a
// quoted field (affects empty_value vs nil_value and strip).
func (p *parser) makeField(s string, started, quoted bool) any {
	if !quoted {
		s = p.stripField(s)
	}
	if !started && !quoted {
		// Truly empty unquoted field → Ruby nil (or nil_value override).
		if p.opts.NilValueSet {
			return p.opts.NilValue
		}
		return nil
	}
	if quoted && s == "" {
		if p.opts.EmptyValueSet {
			return p.opts.EmptyValue
		}
		return ""
	}
	return s
}

// stripField applies Ruby's :strip to an unquoted field.
func (p *parser) stripField(s string) string {
	if p.opts.StripChars != "" {
		return strings.Trim(s, p.opts.StripChars)
	}
	if p.opts.StripSpace {
		return strings.Trim(s, " \t\f\v")
	}
	return s
}

// startsWith reports whether data at p.pos begins with sub.
func (p *parser) startsWith(sub string) bool {
	return p.startsWithAt(p.pos, sub)
}

// startsWithAt reports whether data at i begins with sub.
func (p *parser) startsWithAt(i int, sub string) bool {
	if sub == "" {
		return false
	}
	if i+len(sub) > len(p.data) {
		return false
	}
	return p.data[i:i+len(sub)] == sub
}

// malformed builds an "Illegal quoting" / stray-quote error at the record's
// starting line. MRI reports these at the line where the record began.
func (p *parser) malformed(reason string) error {
	return &MalformedCSVError{Reason: reason, Line: p.line}
}
