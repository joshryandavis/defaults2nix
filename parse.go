package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf16"
)

// parse reads what `defaults read` prints for a domain.
func parse(input string) (value, error) {
	p := &parser{runes: []rune(strings.TrimSpace(input))}
	if p.done() {
		return nil, errors.New("no input")
	}
	v, err := p.value()
	if err != nil {
		return nil, err
	}
	p.skipSpace()
	if !p.done() {
		return nil, fmt.Errorf("unexpected %q after the value", p.peek())
	}
	return v, nil
}

const maxDepth = 100

var errTooDeep = errors.New("value nests too deeply")

type parser struct {
	runes []rune
	pos   int
	depth int
}

func (p *parser) done() bool { return p.pos >= len(p.runes) }
func (p *parser) peek() rune { return p.runes[p.pos] }
func (p *parser) skipSpace() {
	for !p.done() && isSpace(p.peek()) {
		p.pos++
	}
}
func isSpace(r rune) bool { return r == ' ' || r == '\t' || r == '\n' || r == '\r' }

func (p *parser) value() (value, error) {
	p.skipSpace()
	if p.done() {
		return nil, errors.New("expected a value")
	}
	switch p.peek() {
	case '{':
		return p.dict()
	case '(':
		return p.array()
	case '"':
		text, err := p.quoted()
		if err != nil {
			return nil, err
		}
		return scalar(text), nil
	}
	return p.bare()
}

func (p *parser) dict() (value, error) {
	if p.depth++; p.depth > maxDepth {
		return nil, errTooDeep
	}
	defer func() { p.depth-- }()
	p.pos++ // {
	d := dict{}
	for {
		p.skipSpace()
		if p.done() {
			return nil, errors.New("unclosed {")
		}
		if p.peek() == '}' {
			p.pos++
			return d, nil
		}
		key, err := p.key()
		if err != nil {
			return nil, err
		}
		p.skipSpace()
		if p.done() || p.peek() != '=' {
			return nil, fmt.Errorf("expected = after key %q", key)
		}
		p.pos++
		v, err := p.value()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", key, err)
		}
		d = append(d, field{key: key, value: v})
		p.skipSpace()
		// `defaults` separates entries with ; but prints binary blobs with ,.
		if !p.done() && (p.peek() == ';' || p.peek() == ',') {
			p.pos++
		}
	}
}

func (p *parser) array() (value, error) {
	if p.depth++; p.depth > maxDepth {
		return nil, errTooDeep
	}
	defer func() { p.depth-- }()
	p.pos++ // (
	a := array{}
	for {
		p.skipSpace()
		if p.done() {
			return nil, errors.New("unclosed (")
		}
		if p.peek() == ')' {
			p.pos++
			return a, nil
		}
		v, err := p.value()
		if err != nil {
			return nil, err
		}
		a = append(a, v)
		p.skipSpace()
		if !p.done() && (p.peek() == ',' || p.peek() == ';') {
			p.pos++
		}
	}
}

func (p *parser) key() (string, error) {
	if p.peek() == '"' {
		return p.quoted()
	}
	start := p.pos
	for !p.done() && !strings.ContainsRune("=;,}", p.peek()) {
		p.pos++
	}
	key := strings.TrimSpace(string(p.runes[start:p.pos]))
	if key == "" {
		return "", fmt.Errorf("empty key at %q", string(p.runes[start:]))
	}
	return key, nil
}

// bare reads one unquoted token, which may contain spaces in the hex dump that
// `defaults` prints for binary data.
func (p *parser) bare() (value, error) {
	start := p.pos
	for !p.done() && !strings.ContainsRune(",;)}", p.peek()) {
		p.pos++
	}
	text := strings.TrimSpace(string(p.runes[start:p.pos]))
	if text == "" {
		return nil, fmt.Errorf("unexpected %q", p.peek())
	}
	return scalar(text), nil
}

// quoted reads a double-quoted string and decodes the escapes `defaults` uses:
// \" \\ \n \t \r and \Uxxxx, including surrogate pairs.
func (p *parser) quoted() (string, error) {
	p.pos++ // opening quote
	var b strings.Builder
	for {
		if p.done() {
			return "", errors.New("unterminated string")
		}
		r := p.peek()
		if r == '"' {
			p.pos++
			return b.String(), nil
		}
		if r != '\\' {
			b.WriteRune(r)
			p.pos++
			continue
		}
		p.pos++
		if p.done() {
			return "", errors.New("unterminated escape")
		}
		switch e := p.peek(); e {
		case '"', '\\':
			b.WriteRune(e)
			p.pos++
		case 'n':
			b.WriteByte('\n')
			p.pos++
		case 't':
			b.WriteByte('\t')
			p.pos++
		case 'r':
			b.WriteByte('\r')
			p.pos++
		case 'U':
			r, ok := p.hexEscape()
			if !ok {
				b.WriteString(`\U`)
				p.pos++
				continue
			}
			if utf16.IsSurrogate(r) {
				low, ok := p.lowSurrogate()
				if !ok {
					r = '\uFFFD'
				} else {
					r = utf16.DecodeRune(r, low)
				}
			}
			b.WriteRune(r)
		default:
			b.WriteRune('\\')
			b.WriteRune(e)
			p.pos++
		}
	}
}

// hexEscape reads the four hex digits of a \Uxxxx escape at p.pos, leaving p.pos untouched otherwise.
func (p *parser) hexEscape() (rune, bool) {
	if p.pos+5 > len(p.runes) {
		return 0, false
	}
	n, err := strconv.ParseUint(string(p.runes[p.pos+1:p.pos+5]), 16, 32)
	if err != nil {
		return 0, false
	}
	p.pos += 5
	return rune(n), true
}

// lowSurrogate consumes the second half of a surrogate pair, if it follows.
func (p *parser) lowSurrogate() (rune, bool) {
	if p.pos+6 > len(p.runes) || p.peek() != '\\' || p.runes[p.pos+1] != 'U' {
		return 0, false
	}
	n, err := strconv.ParseUint(string(p.runes[p.pos+2:p.pos+6]), 16, 32)
	if err != nil || n < 0xDC00 || n > 0xDFFF {
		return 0, false
	}
	p.pos += 6
	return rune(n), true
}
