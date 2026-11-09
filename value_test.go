package main

import "testing"

func TestNix(t *testing.T) {
	tests := []struct {
		name string
		in   value
		want string
	}{
		{"true", boolean(true), "true"},
		{"false", boolean(false), "false"},
		{"integer", num("42"), "42"},
		{"real", num("0.1"), "0.1"},
		{"string", str("hello"), `"hello"`},
		{"empty string", str(""), `""`},
		{"quote", str(`a"b`), `"a\"b"`},
		{"backslash", str(`a\b`), `"a\\b"`},
		{"interpolation", str("a${b}c"), `"a\${b}c"`},
		{"lone dollar", str("a$b"), `"a$b"`},
		{"newline", str("a\nb"), `"a\nb"`},
		{"tab", str("a\tb"), `"a\tb"`},
		{"unicode", str("café 𝄞"), `"café 𝄞"`},
		{"empty array", array{}, "[]"},
		{"array", array{str("a"), boolean(true), num("1")}, "[\n  \"a\"\n  true\n  1\n]"},
		{"nested array", array{array{str("a")}}, "[\n  [\n    \"a\"\n  ]\n]"},
		{"empty dict", dict{}, "{}"},
		{"dict", dict{{"a", num("1")}, {"b", dict{}}}, "{\n  a = 1;\n  b = {};\n}"},
		{
			"nested dict",
			dict{{"outer", dict{{"inner", str("x")}}}},
			"{\n  outer = {\n    inner = \"x\";\n  };\n}",
		},
		{"plain keys", dict{{"foo", boolean(true)}, {"foo2", boolean(true)}, {"foo'", boolean(true)}},
			"{\n  foo = true;\n  foo2 = true;\n  foo' = true;\n}"},
		{
			"quoted keys",
			dict{
				{"with space", boolean(true)},
				{"with-dash", boolean(true)},
				{"with.dot", boolean(true)},
				{"_under", boolean(true)},
				{"1number", boolean(true)},
				{"with", boolean(true)},
				{"", boolean(true)},
			},
			"{\n  \"with space\" = true;\n  \"with-dash\" = true;\n  \"with.dot\" = true;\n  \"_under\" = true;\n  \"1number\" = true;\n  \"with\" = true;\n  \"\" = true;\n}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.in.nix(0); got != tt.want {
				t.Errorf("nix() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestScalar(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"0", "false"},
		{"1", "true"},
		{"2", "2"},
		{"42", "42"},
		{"007", "7"},
		{"-42", "-42"},
		{"9223372036854775807", "9223372036854775807"},
		{"9223372036854775808", "9223372036854776000.0"},
		{"-9223372036854775808", "-9223372036854776000.0"},
		{"5e-324", `"5e-324"`},
		{"0.5", "0.5"},
		{"-2.75", "-2.75"},
		{"1e+20", "100000000000000000000.0"},
		{"1e-07", "0.0000001"},
		{"hello", `"hello"`},
		{"true", `"true"`},
		{"1.2.3", `"1.2.3"`},
		{".5", `".5"`},
		{"+1", `"+1"`},
		{"NaN", `"NaN"`},
		{"Inf", `"Inf"`},
		{"0x10", `"0x10"`},
		{"", `""`},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := scalar(tt.in).nix(0); got != tt.want {
				t.Errorf("scalar(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
