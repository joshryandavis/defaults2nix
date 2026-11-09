package main

import "testing"

func TestFilter(t *testing.T) {
	tests := []struct {
		name   string
		filter Filter
		in     string
		want   string
	}{
		{
			"binary data is always dropped",
			Filter{},
			`{book = {length = 524, bytes = 0x626f6f6b 0c020000}; kept = 1;}`,
			"{\n  kept = true;\n}",
		},
		{
			"one byte binary data is dropped",
			Filter{},
			`{nl = {length = 1, bytes = 0x0a}; empty = {length = 0, bytes = 0x}; kept = 1;}`,
			"{\n  kept = true;\n}",
		},
		{
			"no filter keeps everything",
			Filter{},
			`{UpdateDate = "2026-11-06 14:42:34 +0000"; "NSWindow Frame Dock" = "1 2 3 4 5 6 7 8"; UUID = "A8604994-4D31-471E-B7F1-D60AC97A287C";}`,
			"{\n  UpdateDate = \"2026-11-06 14:42:34 +0000\";\n  \"NSWindow Frame Dock\" = \"1 2 3 4 5 6 7 8\";\n  UUID = \"A8604994-4D31-471E-B7F1-D60AC97A287C\";\n}",
		},
		{
			"dates: value formats",
			Filter{Dates: true},
			`{a = "2026-11-06 14:42:34 +0000"; b = "2026-11-06T14:42:34Z"; c = "2026-11-06"; d = "not a date"; e = "2026-08-20T15";}`,
			"{\n  d = \"not a date\";\n}",
		},
		{
			"dates: year outside 1900-2100 is kept",
			Filter{Dates: true},
			`{a = "1800-01-01"; b = "2200-01-01";}`,
			"{\n  a = \"1800-01-01\";\n  b = \"2200-01-01\";\n}",
		},
		{
			"dates: keys",
			Filter{Dates: true},
			`{UpdateDate = "x"; checkedAt = 1; savedAt = 2; StartTime = 3; "connected@Display:2" = 4; Category = "y";}`,
			"{\n  Category = \"y\";\n}",
		},
		{
			"dates: inside arrays",
			Filter{Dates: true},
			`{a = ("2026-11-06T14:42:34Z", "kept");}`,
			"{\n  a = [\n    \"kept\"\n  ];\n}",
		},
		{
			"state: keys and values",
			Filter{State: true},
			`{"NSWindow Frame Dock" = "x"; "Main window Frame" = "x"; "image window frame" = "y"; "NSToolbar Configuration Browser" = 1; "appCache" = 2; normal = 3;}`,
			"{\n  normal = 3;\n}",
		},
		{
			"state: rect values",
			Filter{State: true},
			`{a = "{{0, 0}, {100, 100}}"; b = "{40, 40}"; c = "0 0 1920 1080 0 0 1920 1080 "; d = "keep me";}`,
			"{\n  d = \"keep me\";\n}",
		},
		{
			"uuids: values and keys",
			Filter{UUIDs: true},
			`{id = "A8604994-4D31-471E-B7F1-D60AC97A287C"; hashed = "_19a3bc4999bddb89e1a44f4b87bdc37c"; "3906CAB3-0BD4-41A9-8C1E-80F806043E7D" = 1; "not-a-uuid" = "A8604994-4D31-471E-B7F1";}`,
			"{\n  \"not-a-uuid\" = \"A8604994-4D31-471E-B7F1\";\n}",
		},
		{
			"all filters together",
			Filter{Dates: true, State: true, UUIDs: true},
			`{a = "2026-11-06"; b = "{{0, 0}, {1, 1}}"; c = "A8604994-4D31-471E-B7F1-D60AC97A287C"; d = 1;}`,
			"{\n  d = true;\n}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := convert(tt.in, tt.filter)
			if err != nil {
				t.Fatalf("convert(%q): %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("convert(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestFilterDropsWholeDocument(t *testing.T) {
	got, err := convert(`"2026-11-06"`, Filter{Dates: true})
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if got != "{}" {
		t.Errorf("convert() = %q, want {}", got)
	}
}
