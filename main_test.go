package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeSource struct {
	domains    []string
	all        string
	reads      map[string]string
	err        error
	allErr     error
	domainsErr error
}

func (f fakeSource) Domains() ([]string, error) {
	if f.domainsErr != nil {
		return nil, f.domainsErr
	}
	return f.domains, f.err
}

func (f fakeSource) All() (string, error) {
	if f.allErr != nil {
		return "", f.allErr
	}
	return f.all, f.err
}

func (f fakeSource) Read(domain string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	text, ok := f.reads[domain]
	if !ok {
		return "", fmt.Errorf("no such domain %q", domain)
	}
	return text, nil
}

var testSource = fakeSource{
	domains: []string{"com.apple.dock", "Empty Domain", "Broken"},
	all:     "{a = 1;}",
	reads: map[string]string{
		"com.apple.dock": "{autohide = 0;}",
		"Empty Domain":   "{}",
		"Broken":         "{oops",
	},
}

func TestRun(t *testing.T) {
	dockNix := "{\n  autohide = false;\n}\n"

	tests := []struct {
		name      string
		args      func(dir string) []string
		src       *fakeSource
		code      int
		stdout    string
		stdoutHas string
		errText   string
		files     map[string]string
	}{
		{
			name:    "no arguments",
			args:    func(string) []string { return nil },
			code:    2,
			errText: "Usage:",
		},
		{
			name:      "help",
			args:      func(string) []string { return []string{"-h"} },
			stdoutHas: "Usage:",
		},
		{
			name:    "unknown flag",
			args:    func(string) []string { return []string{"-nope"} },
			code:    2,
			errText: "flag provided but not defined",
		},
		{
			name:   "domain to stdout",
			args:   func(string) []string { return []string{"com.apple.dock"} },
			stdout: dockNix,
		},
		{
			name:  "out writes a file",
			args:  func(dir string) []string { return []string{"-out", filepath.Join(dir, "dock.nix"), "com.apple.dock"} },
			files: map[string]string{"dock.nix": dockNix},
		},
		{
			name:  "o is an alias for out",
			args:  func(dir string) []string { return []string{"-o", filepath.Join(dir, "dock.nix"), "com.apple.dock"} },
			files: map[string]string{"dock.nix": dockNix},
		},
		{
			name:   "all",
			args:   func(string) []string { return []string{"-all"} },
			stdout: "{\n  a = true;\n}\n",
		},
		{
			name:   "all keeps domains whose names look like settings",
			args:   func(string) []string { return []string{"-all", "-filter", "dates"} },
			src:    &fakeSource{all: `{"com.apple.FaceTime" = {UpdateDate = "2026-11-06"; Enabled = 1;};}`},
			stdout: "{\n  \"com.apple.FaceTime\" = {\n    Enabled = true;\n  };\n}\n",
		},
		{
			name:    "all takes no domain",
			args:    func(string) []string { return []string{"-all", "com.apple.dock"} },
			code:    2,
			errText: "-all and -split take no domain",
		},
		{
			name:    "all and split together",
			args:    func(string) []string { return []string{"-all", "-split"} },
			code:    2,
			errText: "cannot be combined",
		},
		{
			name:    "split needs out",
			args:    func(string) []string { return []string{"-split"} },
			code:    2,
			errText: "-out is required with -split",
		},
		{
			name:    "split out must be a directory",
			args:    func(dir string) []string { return []string{"-split", "-out", existingFile(t, dir, "file")} },
			code:    2,
			errText: "not a directory",
		},
		{
			name:    "out must be a file without split",
			args:    func(dir string) []string { return []string{"-all", "-out", dir} },
			code:    2,
			errText: "is a directory",
		},
		{
			name:    "out alone is not enough",
			args:    func(dir string) []string { return []string{"-out", filepath.Join(dir, "x.nix")} },
			code:    2,
			errText: "Usage:",
		},
		{
			name:    "unknown filter",
			args:    func(string) []string { return []string{"-filter", "nope", "com.apple.dock"} },
			code:    2,
			errText: "unknown filter",
		},
		{
			name:   "filter names are trimmed",
			args:   func(string) []string { return []string{"-filter", " dates , uuids ", "com.apple.dock"} },
			stdout: dockNix,
		},
		{
			name:    "unknown domain",
			args:    func(string) []string { return []string{"nope"} },
			code:    1,
			errText: `no such domain "nope"`,
		},
		{
			name:    "unparseable domain",
			args:    func(string) []string { return []string{"Broken"} },
			code:    1,
			errText: "expected = after key",
		},
		{
			name:    "flags after the domain are rejected",
			args:    func(dir string) []string { return []string{"com.apple.dock", "-out", filepath.Join(dir, "x.nix")} },
			code:    2,
			errText: "one domain only, flags first",
		},
		{
			name:    "two domains are rejected",
			args:    func(string) []string { return []string{"com.apple.dock", "com.apple.Safari"} },
			code:    2,
			errText: "one domain only, flags first",
		},
		{
			name:    "split writes one file per domain",
			args:    func(dir string) []string { return []string{"-split", "-out", filepath.Join(dir, "out")} },
			files:   map[string]string{"out/com-apple-dock.nix": dockNix},
			errText: "Wrote 1 domains",
		},
		{
			name:    "split reports empty domains",
			args:    func(dir string) []string { return []string{"-split", "-out", filepath.Join(dir, "out")} },
			files:   map[string]string{"out/com-apple-dock.nix": dockNix},
			errText: "skipped 1 empty domains",
		},
		{
			name:    "split reports unreadable domains",
			args:    func(dir string) []string { return []string{"-split", "-out", filepath.Join(dir, "out")} },
			errText: "failed to read or parse: Broken",
			src:     &fakeSource{domains: []string{"com.apple.dock", "Broken"}, reads: testSource.reads},
		},
		{
			name:    "split prefers the first of two colliding names",
			args:    func(dir string) []string { return []string{"-split", "-out", filepath.Join(dir, "out")} },
			errText: "both write com-apple-dock.nix",
			src: &fakeSource{
				domains: []string{"com.apple.dock", "com-apple-dock"},
				reads:   map[string]string{"com.apple.dock": "{a = 1;}", "com-apple-dock": "{b = 1;}"},
			},
			files: map[string]string{"out/com-apple-dock.nix": "{\n  a = true;\n}\n"},
		},
		{
			name: "split reports files it could not write",
			args: func(dir string) []string {
				return []string{"-split", "-out", blockedDir(t, dir, "out", "com-apple-dock.nix")}
			},
			code:    1,
			errText: "nothing was written",
		},
		{
			name:    "split fails when every domain fails",
			args:    func(dir string) []string { return []string{"-split", "-out", filepath.Join(dir, "out")} },
			code:    1,
			errText: "failed to read or parse: Broken",
			src:     &fakeSource{domains: []string{"Broken"}, reads: testSource.reads},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			src := testSource
			if tt.src != nil {
				src = *tt.src
			}
			var stdout, stderr bytes.Buffer
			code := run(tt.args(dir), &stdout, &stderr, src)

			if code != tt.code {
				t.Errorf("run() = %d, want %d\nstderr: %s", code, tt.code, stderr.String())
			}
			if tt.stdoutHas != "" {
				if !strings.Contains(stdout.String(), tt.stdoutHas) {
					t.Errorf("stdout = %q, want it to contain %q", stdout.String(), tt.stdoutHas)
				}
			} else if stdout.String() != tt.stdout {
				t.Errorf("stdout = %q, want %q", stdout.String(), tt.stdout)
			}
			if tt.errText != "" && !strings.Contains(stderr.String(), tt.errText) {
				t.Errorf("stderr = %q, want it to contain %q", stderr.String(), tt.errText)
			}
			for name, want := range tt.files {
				got, err := os.ReadFile(filepath.Join(dir, name))
				if err != nil {
					t.Errorf("%s: %v", name, err)
					continue
				}
				if string(got) != want {
					t.Errorf("%s = %q, want %q", name, got, want)
				}
			}
		})
	}
}

func TestRunSourceErrors(t *testing.T) {
	tests := []struct {
		name string
		args func(dir string) []string
		src  fakeSource
	}{
		{"read", func(string) []string { return []string{"com.apple.dock"} }, fakeSource{err: fmt.Errorf("read boom")}},
		{"all", func(string) []string { return []string{"-all"} }, fakeSource{allErr: fmt.Errorf("all boom")}},
		{"domains", func(dir string) []string { return []string{"-split", "-out", dir} }, fakeSource{domainsErr: fmt.Errorf("domains boom")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := run(tt.args(t.TempDir()), &stdout, &stderr, tt.src); code != 1 {
				t.Errorf("run() = %d, want 1", code)
			}
			if !strings.Contains(stderr.String(), "boom") {
				t.Errorf("stderr = %q, want the source error", stderr.String())
			}
		})
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := map[string]string{
		"com.apple.Safari":        "com-apple-Safari",
		"Custom User Preferences": "Custom_User_Preferences",
		"path/to/domain":          "path_to_domain",
		"NSGlobalDomain":          "NSGlobalDomain",
	}
	for domain, want := range tests {
		if got := sanitizeFilename(domain); got != want {
			t.Errorf("sanitizeFilename(%q) = %q, want %q", domain, got, want)
		}
	}
}

func existingFile(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// blockedDir returns an output directory where name already exists as a directory,
// so writing that file fails.
func blockedDir(t *testing.T, dir, out, name string) string {
	t.Helper()
	path := filepath.Join(dir, out)
	if err := os.MkdirAll(filepath.Join(path, name), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}
