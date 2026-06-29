// Copyright (c) the go-ruby-csv/csv authors
//
// SPDX-License-Identifier: BSD-3-Clause

package csv

import (
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

// rubyBin locates a usable `ruby` whose CSV library is recent enough to match
// this port (MRI ≥ 4.0). The oracle tests skip themselves when ruby is absent
// (the qemu cross-arch lanes and the Windows lane), so the deterministic suite
// alone drives the 100% coverage gate there.
func rubyBin(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("ruby")
	if err != nil {
		t.Skip("ruby not on PATH; skipping MRI oracle")
	}
	out, err := exec.Command(path, "-e", "print RUBY_VERSION").Output()
	if err != nil {
		t.Skipf("ruby version probe failed: %v", err)
	}
	if !versionAtLeast(string(out), 4, 0) {
		t.Skipf("ruby %s < 4.0; skipping MRI oracle", out)
	}
	return path
}

// versionAtLeast reports whether a "MAJOR.MINOR..." string is ≥ major.minor.
func versionAtLeast(v string, major, minor int) bool {
	parts := strings.Split(strings.TrimSpace(v), ".")
	if len(parts) < 2 {
		return false
	}
	maj, err1 := strconv.Atoi(parts[0])
	min, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return false
	}
	if maj != major {
		return maj > major
	}
	return min >= minor
}

// rubyCSV runs a Ruby CSV script, feeding the CSV input over *binmode stdin* so
// embedded "\n"/"\r\n" survive Windows text-mode (CSV is newline-sensitive — the
// go-ruby-erb Windows lesson, sharpened here). The script reads STDIN.read for
// the raw bytes and $stdout.binmode for clean output.
func rubyCSV(t *testing.T, bin, input, script string) string {
	t.Helper()
	full := "$stdout.binmode\n$stdin.binmode\nINPUT = $stdin.read\n" + script
	cmd := exec.Command(bin, "-rcsv", "-e", full)
	cmd.Stdin = strings.NewReader(input)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ruby error: %v\nscript:\n%s\noutput:\n%s", err, script, out)
	}
	return string(out)
}

// inspectValue renders a parsed Go field the way Ruby's p / inspect would, so we
// can compare our parse against MRI's `p CSV.parse_line(...)` byte-for-byte.
func inspectValue(v any) string {
	switch x := v.(type) {
	case nil:
		return "nil"
	case string:
		return rubyStringInspect(x)
	case int:
		return strconv.Itoa(x)
	case float64:
		return formatFloat(x)
	case Symbol:
		return ":" + string(x)
	default:
		return "?"
	}
}

// rubyStringInspect mirrors String#inspect for the bytes our corpus uses.
func rubyStringInspect(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteByte(c)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// inspectRow renders a []any as Ruby would `p` an Array.
func inspectRow(row []any) string {
	parts := make([]string, len(row))
	for i, v := range row {
		parts[i] = inspectValue(v)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// TestOracleParseLine differentially checks ParseLine against CSV.parse_line over
// a broad corpus of quoting, separators and converter cases.
func TestOracleParseLine(t *testing.T) {
	bin := rubyBin(t)
	cases := []struct {
		in   string
		opts Options
		ropt string // Ruby option literal, e.g. ", converters: [:integer]"
	}{
		{in: "a,b,c"},
		{in: `a,"b,c",d`},
		{in: `a,"b""c",d`},
		{in: "a,,c"},
		{in: ","},
		{in: `"a,b"` + "\n" + `c`}, // newline inside quotes
		{in: `"x` + "\n" + `y",z`},
		{in: "a;b;c", opts: Options{ColSep: ";"}, ropt: `, col_sep: ";"`},
		{in: "1,2.5,foo", opts: Options{Converters: []string{"integer"}}, ropt: ", converters: [:integer]"},
		{in: "1,2.5,foo", opts: Options{Converters: []string{"float"}}, ropt: ", converters: [:float]"},
		{in: "1,2.5,foo", opts: Options{Converters: []string{"numeric"}}, ropt: ", converters: [:numeric]"},
		{in: "007,12abc", opts: Options{Converters: []string{"integer"}}, ropt: ", converters: [:integer]"},
		{in: "0x10,1_000", opts: Options{Converters: []string{"integer"}}, ropt: ", converters: [:integer]"},
		{in: "  a  ,  b  ", opts: Options{StripSpace: true}, ropt: ", strip: true"},
	}
	for _, c := range cases {
		got, err := ParseLine(c.in, c.opts)
		if err != nil {
			t.Fatalf("ParseLine(%q): %v", c.in, err)
		}
		want := strings.TrimRight(rubyCSV(t, bin, c.in,
			"p CSV.parse_line(INPUT"+c.ropt+")"), "\n")
		if inspectRow(got) != want {
			t.Errorf("parse_line %q%s:\n  go   = %s\n  ruby = %s", c.in, c.ropt, inspectRow(got), want)
		}
	}
}

// TestOracleGenerateLine differentially checks GenerateLine against
// CSV.generate_line. The row is built in Ruby and in Go from the same literals.
func TestOracleGenerateLine(t *testing.T) {
	bin := rubyBin(t)
	cases := []struct {
		row  []any
		ruby string // Ruby array literal
		opts Options
		ropt string
	}{
		{row: []any{"a", "b", "c"}, ruby: `["a","b","c"]`},
		{row: []any{"a", "b,c", "d"}, ruby: `["a","b,c","d"]`},
		{row: []any{"a", `b"c`, "d"}, ruby: `["a",%q{b"c},"d"]`},
		{row: []any{"a", "x\ny", "c"}, ruby: `["a","x\ny","c"]`},
		{row: []any{1, 2.5, nil, "x"}, ruby: `[1,2.5,nil,"x"]`},
		{row: []any{"", "b"}, ruby: `["","b"]`},
		{row: []any{"a", "b"}, ruby: `["a","b"]`, opts: Options{ForceQuotes: true}, ropt: ", force_quotes: true"},
		{row: []any{"", "b"}, ruby: `["","b"]`, opts: Options{QuoteEmptySet: true, QuoteEmpty: false}, ropt: ", quote_empty: false"},
	}
	for _, c := range cases {
		got, err := GenerateLine(c.row, c.opts)
		if err != nil {
			t.Fatalf("GenerateLine: %v", err)
		}
		// Ruby prints the generated line; we compare raw bytes.
		want := rubyCSV(t, bin, "",
			"print CSV.generate_line("+c.ruby+c.ropt+")")
		if got != want {
			t.Errorf("generate_line %s%s:\n  go   = %q\n  ruby = %q", c.ruby, c.ropt, got, want)
		}
	}
}

// TestOracleMalformed checks our MalformedCSVError message + line number match
// MRI's CSV::MalformedCSVError exactly.
func TestOracleMalformed(t *testing.T) {
	bin := rubyBin(t)
	inputs := []string{
		`a,"b`,
		"a,\"b\nc,d",
		`a,b"c,d`,
		`a,"b"x,d`,
		"a,b\nc,\"d\ne",
		"a,b\nc,d\"e",
		"r1,x\nr2,y\nr3,\"open\nmore",
	}
	for _, in := range inputs {
		_, err := Parse(in, Options{})
		me, ok := err.(*MalformedCSVError)
		if !ok {
			t.Fatalf("Parse(%q) err = %v, want malformed", in, err)
		}
		want := strings.TrimRight(rubyCSV(t, bin, in,
			"begin; CSV.parse(INPUT); rescue CSV::MalformedCSVError => e; print e.message; end"), "\n")
		if me.Error() != want {
			t.Errorf("malformed %q:\n  go   = %q\n  ruby = %q", in, me.Error(), want)
		}
	}
}

// TestOracleHeaders checks the Table/Row model against CSV.parse(headers: true).
func TestOracleHeaders(t *testing.T) {
	bin := rubyBin(t)
	in := "name,age\nAlice,30\nBob,25"
	v, err := Parse(in, Options{Headers: true})
	if err != nil {
		t.Fatal(err)
	}
	tbl := v.(*Table)

	// to_a equivalence.
	gotA := tbl.ToArray()
	wantA := rubyCSV(t, bin, in, "p CSV.parse(INPUT, headers: true).to_a")
	gotAStr := "["
	for i, r := range gotA {
		if i > 0 {
			gotAStr += ", "
		}
		gotAStr += inspectRow(r)
	}
	gotAStr += "]"
	if gotAStr != strings.TrimRight(wantA, "\n") {
		t.Errorf("headers to_a:\n  go   = %s\n  ruby = %s", gotAStr, wantA)
	}

	// by-name field access.
	wantField := strings.TrimRight(rubyCSV(t, bin, in,
		`p CSV.parse(INPUT, headers: true)[0]["name"]`), "\n")
	got, _ := tbl.Rows[0].Field("name")
	if inspectValue(got) != wantField {
		t.Errorf("by-name: go=%s ruby=%s", inspectValue(got), wantField)
	}
}

// TestOracleHeaderConverters checks :symbol header conversion against MRI.
func TestOracleHeaderConverters(t *testing.T) {
	bin := rubyBin(t)
	in := "First Name,Last-Name!,  Age  \nAlice,Smith,30"
	v, _ := Parse(in, Options{Headers: true, HeaderConverters: []string{"symbol"}})
	tbl := v.(*Table)
	wantH := strings.TrimRight(rubyCSV(t, bin, in,
		"p CSV.parse(INPUT, headers: true, header_converters: [:symbol]).headers"), "\n")
	gotH := "["
	for i, h := range tbl.Headers {
		if i > 0 {
			gotH += ", "
		}
		gotH += inspectValue(h)
	}
	gotH += "]"
	if gotH != wantH {
		t.Errorf("symbol headers:\n  go   = %s\n  ruby = %s", gotH, wantH)
	}
}

// TestOracleRoundTrip generates with Go, parses with MRI, and confirms the rows
// survive the trip identically.
func TestOracleRoundTrip(t *testing.T) {
	bin := rubyBin(t)
	rows := [][]any{
		{"a", "b,c", "d\ne"},
		{`x"y`, "", "z"},
		{"plain", "with space", "trailing,"},
	}
	data, err := Generate(rows, Options{})
	if err != nil {
		t.Fatal(err)
	}
	want := strings.TrimRight(rubyCSV(t, bin, data, "p CSV.parse(INPUT)"), "\n")
	got := "["
	for i, r := range rows {
		if i > 0 {
			got += ", "
		}
		got += inspectRow(r)
	}
	got += "]"
	if got != want {
		t.Errorf("round trip:\n  go   = %s\n  ruby = %s\n  data = %q", got, want, data)
	}
}
