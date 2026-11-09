package main

import (
	"os"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty dict", `{}`, "{}"},
		{"empty array", `()`, "[]"},
		{"entry", `{a = 1;}`, "{\n  a = true;\n}"},
		{"last entry without semicolon", `{a = 1}`, "{\n  a = true;\n}"},
		{"no spaces around equals", `{a=1;}`, "{\n  a = true;\n}"},
		{"comma separated", `{a = 1, b = 2}`, "{\n  a = true;\n  b = 2;\n}"},
		{"quoted key", `{"a b" = x;}`, "{\n  \"a b\" = \"x\";\n}"},
		{"nested", `{a = {b = (1, "x y");};}`, "{\n  a = {\n    b = [\n      true\n      \"x y\"\n    ];\n  };\n}"},
		{"array trailing comma", `(a, b,)`, "[\n  \"a\"\n  \"b\"\n]"},
		{"array semicolons", `(a; b)`, "[\n  \"a\"\n  \"b\"\n]"},
		{"escaped quote", `"a\"b"`, `"a\"b"`},
		{"escaped backslash", `"a\\b"`, `"a\\b"`},
		{"newline escape", `"a\nb"`, `"a\nb"`},
		{"tab escape", `"a\tb"`, `"a\tb"`},
		{"unicode escape", `"caf\U00e9"`, `"café"`},
		{"surrogate pair", `"\Ud834\Udd1e"`, `"𝄞"`},
		{"lone surrogate", `"\Ud834"`, `"�"`},
		{"unknown escape kept", `"a\qb"`, `"a\\qb"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := parse(tt.in)
			if err != nil {
				t.Fatalf("parse(%q): %v", tt.in, err)
			}
			if got := v.nix(0); got != tt.want {
				t.Errorf("parse(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name string
		in   string
	}{
		{"empty", ""},
		{"only spaces", "  \n "},
		{"unclosed dict", `{a = 1;`},
		{"unclosed array", `(a, b`},
		{"unterminated string", `"abc`},
		{"missing key", `{= 1;}`},
		{"missing value", `{a = ;}`},
		{"missing equals", `{a 1;}`},
		{"trailing junk", `{} {}`},
		{"too deep", strings.Repeat("{a = ", 200) + "1" + strings.Repeat(";}", 200)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if v, err := parse(tt.in); err == nil {
				t.Errorf("parse(%q) = %q, want an error", tt.in, v.nix(0))
			}
		})
	}
}

// TestGolden pins the output for a fixture trimmed from `defaults read
// com.apple.dock`, plus the typed values that text format makes awkward.
func TestGolden(t *testing.T) {
	raw, err := os.ReadFile("testdata/defaults.txt")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		golden string
		filter Filter
	}{
		{"unfiltered", "testdata/defaults.nix", Filter{}},
		{"filtered", "testdata/defaults.filtered.nix", Filter{Dates: true, State: true, UUIDs: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want, err := os.ReadFile(tt.golden)
			if err != nil {
				t.Fatal(err)
			}
			got, err := convert(string(raw), tt.filter)
			if err != nil {
				t.Fatalf("convert: %v", err)
			}
			if strings.TrimSpace(got) != strings.TrimSpace(string(want)) {
				t.Errorf("convert(defaults.txt) =\n%s\nwant:\n%s", got, want)
			}
		})
	}
}
