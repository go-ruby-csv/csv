<p align="center"><img src="https://raw.githubusercontent.com/go-ruby-csv/brand/main/social/go-ruby-csv-csv.png" alt="go-ruby-csv/csv" width="720"></p>

# csv — go-ruby-csv

[![Docs](https://img.shields.io/badge/docs-mkdocs--material-DC2626)](https://go-ruby-csv.github.io/docs/)
[![License](https://img.shields.io/badge/license-BSD--3--Clause-blue)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.26.4%2B-00ADD8)](https://go.dev/dl/)
[![Coverage](https://img.shields.io/badge/coverage-100%25-1a7f37)](#tests--coverage)

**A pure-Go (no cgo) reimplementation of Ruby's [`CSV`](https://docs.ruby-lang.org/en/master/CSV.html)
standard library** — the parser and generator behind MRI 4.0.5's `csv` gem
(3.3.5). It parses and generates CSV/TSV with Ruby's rich option set, reproducing
MRI semantics byte-for-byte: the dialect options (`col_sep` / `row_sep` with
`:auto` detection / `quote_char`), RFC-style quoting with embedded quotes,
separators and newlines inside quoted fields, the headers model
(`CSV::Row` / `CSV::Table`), the built-in field and header converters, and
`CSV::MalformedCSVError` with MRI-exact messages and line numbers — **without any
Ruby runtime.**

It is the CSV backend for
[go-embedded-ruby](https://github.com/go-embedded-ruby/ruby), but is a
**standalone, reusable** module with no dependency on the Ruby runtime — a sibling
of [go-ruby-regexp](https://github.com/go-ruby-regexp/regexp) (the Onigmo engine),
[go-ruby-erb](https://github.com/go-ruby-erb/erb) (the ERB compiler) and
[go-ruby-yaml](https://github.com/go-ruby-yaml/yaml) (the Psych port).

> **What it is — and isn't.** Parsing and generating CSV for the Ruby dialect
> (quoting, separators, converters, the headers model, the malformed-error
> grammar) is fully deterministic and needs **no interpreter**, so it lives here
> as pure Go. Binding the parsed fields to live Ruby objects — instantiating a
> `Date`, a `Symbol`, a `CSV::Row` — is the host's job; this library hands back a
> small, explicit value model (`nil`, `string`, `int`, `float64`, `Date`,
> `DateTime`, `Symbol`, `*Row`, `*Table`) the host maps to and from its own
> objects.

## Features

Faithful port of MRI's CSV parser and writer, validated against the `ruby`
binary on every supported platform:

- **Parsing** — `ParseLine` (`CSV.parse_line`), `Parse` (`CSV.parse`) and
  `ParseRows`. RFC-style quoting with `""` escapes; separators and newlines
  inside quoted fields; empty unquoted fields become `nil`, empty lines become
  `[]`, an empty input line yields `nil` — exactly as MRI.
- **Dialect options** — `col_sep` (multi-byte), `row_sep` (explicit or `:auto`
  detection of `\r\n` / `\r` / `\n`), `quote_char` (or disabled via
  `NoQuote`).
- **Headers** — `headers: true` / `"first_row"` / `[]string`, producing a
  `Table` of `Row`s with access by header name or index, plus `return_headers`
  and `write_headers`.
- **Field converters** — `:integer`, `:float`, `:numeric`, `:date`,
  `:date_time`, `:time`, `:all`, with MRI-faithful `Integer()` / `Float()`
  acceptance (radix prefixes, `_` separators, octal leading zero, hex floats)
  and the exact `DateMatcher` / `DateTimeMatcher` shapes.
- **Header converters** — `:downcase`, `:symbol`, `:symbol_raw`.
- **`skip_blanks`, `skip_lines`, `liberal_parsing`, `strip`, `nil_value`,
  `empty_value`** parse options.
- **Generation** — `GenerateLine` (`CSV.generate_line`) and `Generate`
  (`CSV.generate`) with `force_quotes`, `quote_empty`, and the precise
  quote-when-needed rule (quote char, separators, CR or LF).
- **`MalformedCSVError`** — `Unclosed quoted field`, `Illegal quoting`,
  `Any value after quoted field isn't allowed`, each with MRI-exact wording and
  1-based line numbers.

## Usage

```go
import "github.com/go-ruby-csv/csv"

// Parse a record (CSV.parse_line).
row, _ := csv.ParseLine(`a,"b,c",d`, csv.Options{})
// row == []any{"a", "b,c", "d"}

// Empty unquoted fields are nil; embedded newlines survive in quotes.
row, _ = csv.ParseLine("a,,\"x\ny\"", csv.Options{})
// row == []any{"a", nil, "x\ny"}

// Converters.
row, _ = csv.ParseLine("1,2.5,foo", csv.Options{Converters: []string{"numeric"}})
// row == []any{1, 2.5, "foo"}

// Headers → Table / Row.
t, _ := csv.Parse("name,age\nAlice,30", csv.Options{Headers: true})
tbl := t.(*csv.Table)
v, _ := tbl.Rows[0].Field("name") // "Alice"

// Generate (CSV.generate_line).
line, _ := csv.GenerateLine([]any{"a", "b,c", nil}, csv.Options{})
// line == "a,\"b,c\",\n"

// Malformed input yields a CSV::MalformedCSVError-compatible error.
_, err := csv.Parse("a,\"b\nc,d", csv.Options{})
// err.Error() == "Unclosed quoted field in line 1."
```

## Tests &amp; coverage

The suite is split into a deterministic, Ruby-free corpus (round-trip
parse/generate, quoting edge cases, headers, converters, malformed errors) and a
differential **MRI oracle** that runs every case against the `ruby` binary on the
ubuntu/macos lanes. The oracle gates itself on `RUBY_VERSION >= 4.0` and feeds
CSV inputs over **binmode stdin** so embedded `\n` / `\r\n` are never CRLF-mangled
on Windows. The deterministic tests alone hold **100.0% coverage**, so the
no-ruby, Windows and qemu cross-arch lanes all pass the gate.

```sh
go test -race -coverprofile=cover.out ./...
go tool cover -func=cover.out | tail -1   # total: 100.0%
```

CGO is never used; the library builds and tests clean on all six supported
64-bit targets (amd64, arm64, riscv64, loong64, ppc64le, and big-endian s390x).

## License

BSD-3-Clause — see [LICENSE](LICENSE). Copyright the go-ruby-csv/csv authors.

## WebAssembly

Being pure Go (CGO=0), this library also compiles to **WebAssembly** — both
`GOOS=js GOARCH=wasm` (browser / Node.js) and `GOOS=wasip1 GOARCH=wasm` (WASI).
CI builds both targets on every push, alongside the six 64-bit native/qemu arches.

```sh
GOOS=js     GOARCH=wasm go build ./...   # browser / Node
GOOS=wasip1 GOARCH=wasm go build ./...   # WASI (wasmtime, wasmer, wasmedge, …)
```
