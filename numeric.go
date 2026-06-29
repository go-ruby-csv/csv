// Copyright (c) the go-ruby-csv/csv authors
//
// SPDX-License-Identifier: BSD-3-Clause

package csv

import (
	"strconv"
	"strings"
)

// rubyInteger mirrors Ruby's Kernel#Integer for strings, as the :integer
// converter uses it (Integer(f) rescue f). It accepts optional surrounding
// whitespace, an optional sign, an optional 0x/0o/0b/0d radix prefix (and bare
// leading 0 as octal), and `_` digit separators (never leading, trailing, or
// doubled). On any deviation it reports ok=false so the field stays a string.
func rubyInteger(s string) (int, bool) {
	t := strings.TrimFunc(s, isRubySpace)
	if t == "" {
		return 0, false
	}
	neg := false
	switch t[0] {
	case '+':
		t = t[1:]
	case '-':
		neg = true
		t = t[1:]
	}
	if t == "" {
		return 0, false
	}

	base := 10
	hasPrefix := false
	if len(t) >= 2 && t[0] == '0' {
		switch t[1] {
		case 'x', 'X':
			base, t, hasPrefix = 16, t[2:], true
		case 'o', 'O':
			base, t, hasPrefix = 8, t[2:], true
		case 'b', 'B':
			base, t, hasPrefix = 2, t[2:], true
		case 'd', 'D':
			base, t, hasPrefix = 10, t[2:], true
		default:
			// Bare leading zero (e.g. "017") is octal in Ruby's Integer().
			base = 8
		}
	}
	if hasPrefix && t == "" {
		return 0, false
	}

	digits, ok := stripUnderscores(t)
	if !ok || digits == "" {
		return 0, false
	}
	n, err := strconv.ParseInt(digits, base, 64)
	if err != nil {
		return 0, false
	}
	if neg {
		n = -n
	}
	return int(n), true
}

// stripUnderscores removes Ruby's `_` digit separators, rejecting leading,
// trailing, or consecutive underscores (Integer/Float both forbid those).
func stripUnderscores(s string) (string, bool) {
	if s == "" {
		return "", true
	}
	if s[0] == '_' || s[len(s)-1] == '_' {
		return "", false
	}
	var b strings.Builder
	prevUnderscore := false
	for i := 0; i < len(s); i++ {
		if s[i] == '_' {
			if prevUnderscore {
				return "", false
			}
			prevUnderscore = true
			continue
		}
		prevUnderscore = false
		b.WriteByte(s[i])
	}
	return b.String(), true
}

// rubyFloat mirrors Ruby's Kernel#Float for strings, as the :float converter
// uses it (Float(f) rescue f). It accepts optional surrounding whitespace, an
// optional sign, decimal floats (including a bare leading or trailing dot),
// scientific notation, hexadecimal floats (0x...p...), and `_` separators. It
// rejects "inf"/"nan" (Ruby's Float() does, unlike strconv).
func rubyFloat(s string) (float64, bool) {
	t := strings.TrimFunc(s, isRubySpace)
	if t == "" {
		return 0, false
	}
	clean, ok := stripUnderscores(t)
	if !ok {
		return 0, false
	}
	// Reject the strconv-accepted but Ruby-rejected spellings.
	low := strings.ToLower(clean)
	low = strings.TrimLeft(low, "+-")
	if low == "inf" || low == "infinity" || low == "nan" {
		return 0, false
	}
	if !looksLikeRubyFloat(clean) {
		return 0, false
	}
	f, err := strconv.ParseFloat(clean, 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

// looksLikeRubyFloat gate-keeps the grammar Ruby's Float() accepts before
// handing the (underscore-stripped) text to strconv. It permits a leading or
// trailing dot (".5", "5.") and hex floats, both of which strconv also accepts,
// while excluding stray characters strconv might tolerate differently.
func looksLikeRubyFloat(s string) bool {
	if s == "" {
		return false
	}
	body := s
	if body[0] == '+' || body[0] == '-' {
		body = body[1:]
	}
	if body == "" {
		return false
	}
	if len(body) >= 2 && body[0] == '0' && (body[1] == 'x' || body[1] == 'X') {
		// Hex float: delegate the detailed grammar to strconv (it matches
		// Ruby's acceptance for the 0x...p... form).
		return true
	}
	hasDigit := false
	hasDot := false
	hasExp := false
	for i := 0; i < len(body); i++ {
		c := body[i]
		switch {
		case c >= '0' && c <= '9':
			hasDigit = true
		case c == '.':
			if hasDot || hasExp {
				return false
			}
			hasDot = true
		case c == 'e' || c == 'E':
			if hasExp || !hasDigit {
				return false
			}
			hasExp = true
			if i+1 < len(body) && (body[i+1] == '+' || body[i+1] == '-') {
				i++
			}
		default:
			return false
		}
	}
	return hasDigit
}
