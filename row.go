// Copyright (c) the go-ruby-csv/csv authors
//
// SPDX-License-Identifier: BSD-3-Clause

package csv

// Row mirrors CSV::Row: an ordered list of fields paired with the table's
// headers, offering access by header name or by index. It is the per-record type
// the binding maps to a CSV::Row.
type Row struct {
	// Headers are the header values (string or Symbol after header conversion),
	// shared with the owning Table.
	Headers []any
	// Fields are this row's values, positionally aligned with Headers.
	Fields []any
	// HeaderRow reports whether this Row is the header row itself (emitted only
	// when return_headers is set), mirroring CSV::Row#header_row?.
	HeaderRow bool
}

// Field returns the value for the given header name, mirroring Row#[] with a
// header argument. When a header repeats, the first match wins (as in MRI).
// ok is false when no such header exists.
func (r *Row) Field(name any) (any, bool) {
	for i, h := range r.Headers {
		if headerEqual(h, name) {
			if i < len(r.Fields) {
				return r.Fields[i], true
			}
			return nil, true
		}
	}
	return nil, false
}

// At returns the value at index i, mirroring Row#[] with an integer argument.
// A negative index counts from the end (Ruby semantics). ok is false when out of
// range.
func (r *Row) At(i int) (any, bool) {
	if i < 0 {
		i += len(r.Fields)
	}
	if i < 0 || i >= len(r.Fields) {
		return nil, false
	}
	return r.Fields[i], true
}

// ToHash returns the row as an ordered header→value pairing, mirroring
// Row#to_h. When a header repeats, the last value wins (MRI's to_h behaviour).
func (r *Row) ToHash() map[any]any {
	m := make(map[any]any, len(r.Headers))
	for i, h := range r.Headers {
		if i < len(r.Fields) {
			m[h] = r.Fields[i]
		} else {
			m[h] = nil
		}
	}
	return m
}

// headerEqual compares a stored header against a lookup key, treating a string
// and a Symbol of the same text as distinct (matching MRI, where "a" != :a),
// but allowing the caller to pass either type explicitly.
func headerEqual(stored, key any) bool {
	return stored == key
}

// Table mirrors CSV::Table: a header row plus its data Rows.
type Table struct {
	// Headers are the (possibly converted) header values.
	Headers []any
	// Rows are the data rows. When ReturnHeaders was set, the first element is
	// the header Row (HeaderRow true).
	Rows []*Row
}

// Row returns the i-th data row, mirroring Table#[] with an integer.
func (t *Table) Row(i int) (*Row, bool) {
	if i < 0 {
		i += len(t.Rows)
	}
	if i < 0 || i >= len(t.Rows) {
		return nil, false
	}
	return t.Rows[i], true
}

// ToArray renders the table as Ruby's CSV::Table#to_a: the header row followed
// by each data row's fields. When the header Row is already present (return_
// headers), it is not duplicated.
func (t *Table) ToArray() [][]any {
	out := make([][]any, 0, len(t.Rows)+1)
	headerEmitted := false
	for _, r := range t.Rows {
		if r.HeaderRow {
			headerEmitted = true
		}
		out = append(out, r.Fields)
	}
	if !headerEmitted {
		hdr := make([]any, len(t.Headers))
		copy(hdr, t.Headers)
		out = append([][]any{hdr}, out...)
	}
	return out
}

// buildTable assembles a Table from raw parsed rows under the chosen header
// policy, applying header and field converters exactly as CSV does.
func buildTable(rows [][]any, opts Options) (*Table, error) {
	var headers []any
	var dataStart int

	switch h := opts.Headers.(type) {
	case []string:
		headers = make([]any, len(h))
		for i, s := range h {
			headers[i] = s
		}
		dataStart = 0
	default: // true / "first_row"
		if len(rows) == 0 {
			return &Table{Headers: nil, Rows: nil}, nil
		}
		first := rows[0]
		headers = make([]any, len(first))
		copy(headers, first)
		dataStart = 1
	}

	// Apply header converters.
	for i := range headers {
		headers[i] = applyHeaderConverters(headers[i], opts.HeaderConverters)
	}

	t := &Table{Headers: headers}

	// return_headers: emit the header row first.
	if opts.ReturnHeaders {
		hfields := make([]any, len(headers))
		copy(hfields, headers)
		t.Rows = append(t.Rows, &Row{Headers: headers, Fields: hfields, HeaderRow: true})
	}

	for _, raw := range rows[dataStart:] {
		fields := make([]any, len(raw))
		copy(fields, raw)
		applyConverters(fields, opts.Converters)
		t.Rows = append(t.Rows, &Row{Headers: headers, Fields: fields})
	}
	return t, nil
}
