package main

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

// value is one parsed `defaults` value that knows how to render itself as Nix.
type value interface {
	nix(indent int) string
}

type str string

func (s str) nix(int) string { return quote(string(s)) }

// num holds an already valid Nix numeric literal.
type num string

func (n num) nix(int) string { return string(n) }

type boolean bool

func (b boolean) nix(int) string {
	if b {
		return "true"
	}
	return "false"
}

type array []value

func (a array) nix(indent int) string {
	if len(a) == 0 {
		return "[]"
	}
	var b strings.Builder
	b.WriteString("[\n")
	for _, v := range a {
		b.WriteString(pad(indent + 1))
		b.WriteString(v.nix(indent + 1))
		b.WriteString("\n")
	}
	b.WriteString(pad(indent))
	b.WriteString("]")
	return b.String()
}

type dict []field

type field struct {
	key   string
	value value
}

func (d dict) nix(indent int) string {
	if len(d) == 0 {
		return "{}"
	}
	var b strings.Builder
	b.WriteString("{\n")
	for _, f := range d {
		b.WriteString(pad(indent + 1))
		b.WriteString(attrName(f.key))
		b.WriteString(" = ")
		b.WriteString(f.value.nix(indent + 1))
		b.WriteString(";\n")
	}
	b.WriteString(pad(indent))
	b.WriteString("}")
	return b.String()
}

func pad(indent int) string { return strings.Repeat("  ", indent) }

// quote renders s as a Nix double-quoted string.
func quote(s string) string {
	runes := []rune(s)
	var b strings.Builder
	b.WriteByte('"')
	for i, r := range runes {
		switch r {
		case '\\', '"':
			b.WriteByte('\\')
			b.WriteRune(r)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		case '\r':
			b.WriteString(`\r`)
		case '$':
			if i+1 < len(runes) && runes[i+1] == '{' {
				b.WriteByte('\\')
			}
			b.WriteByte('$')
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// nixKeywords cannot be used as bare attribute names.
var nixKeywords = map[string]bool{
	"assert": true, "else": true, "if": true, "in": true, "inherit": true,
	"let": true, "rec": true, "then": true, "with": true,
}

func attrName(key string) string {
	if plainAttr(key) && !nixKeywords[key] {
		return key
	}
	return quote(key)
}

// plainAttr reports whether key can be written without quotes in Nix.
// Dashes, underscores and digits at the start are legal there but `defaults`
// quotes them, and quoting keeps the output close to what it prints.
func plainAttr(key string) bool {
	for i, r := range key {
		if i == 0 {
			if !('a' <= r && r <= 'z' || 'A' <= r && r <= 'Z') {
				return false
			}
			continue
		}
		switch {
		case r == '\'':
		case 'a' <= r && r <= 'z', 'A' <= r && r <= 'Z', '0' <= r && r <= '9':
		default:
			return false
		}
	}
	return key != ""
}

var decimalLiteral = regexp.MustCompile(`^-?[0-9]+(\.[0-9]+)?([eE][-+]?[0-9]+)?$`)

// scalar turns one `defaults` token into a value. `defaults` prints booleans as
// 1 and 0, numbers as plain decimal, and everything else as a string.
func scalar(text string) value {
	switch text {
	case "1":
		return boolean(true)
	case "0":
		return boolean(false)
	}
	if !decimalLiteral.MatchString(text) {
		return str(text)
	}
	// Nix cannot parse -9223372036854775808: it reads the sign as an operator.
	if i, err := strconv.ParseInt(text, 10, 64); err == nil && i != math.MinInt64 {
		return num(strconv.FormatInt(i, 10))
	}
	f, err := strconv.ParseFloat(text, 64)
	if err != nil || isSubnormal(f) {
		// Nix rejects subnormal floats, so the text has to stay a string.
		return str(text)
	}
	return num(nixFloat(f))
}

func isSubnormal(f float64) bool { return f != 0 && math.Abs(f) < 0x1p-1022 }

// nixFloat renders f as a Nix float: Nix requires a dot in the mantissa.
func nixFloat(f float64) string {
	s := strconv.FormatFloat(f, 'f', -1, 64)
	if !strings.ContainsRune(s, '.') {
		s += ".0"
	}
	return s
}
