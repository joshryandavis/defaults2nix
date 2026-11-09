package main

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Filter drops values that are not worth keeping in a Nix configuration.
type Filter struct {
	Dates bool
	State bool
	UUIDs bool
}

// drop reports whether the field at key, or the whole value, should be dropped.
func (f Filter) drop(key string, v value) bool {
	if isBinary(v) {
		return true
	}
	s, isString := v.(str)
	if f.Dates && (isTimestampKey(key) || isString && isDateString(string(s))) {
		return true
	}
	if f.State && (isUIStateKey(key) || isString && isUIStateValue(string(s))) {
		return true
	}
	if f.UUIDs && (isUUIDKey(key) || isString && (isUUID(string(s)) || isHashedID(string(s)))) {
		return true
	}
	return false
}

// applyDomains filters each domain's settings while keeping every domain: a
// domain name is not a setting.
func (f Filter) applyDomains(v value) value {
	d, ok := v.(dict)
	if !ok {
		return f.apply(v)
	}
	out := make(dict, 0, len(d))
	for _, fld := range d {
		out = append(out, field{key: fld.key, value: f.apply(fld.value)})
	}
	return out
}

// apply removes the dropped parts of v.
func (f Filter) apply(v value) value {
	switch v := v.(type) {
	case dict:
		out := make(dict, 0, len(v))
		for _, fld := range v {
			if f.drop(fld.key, fld.value) {
				continue
			}
			out = append(out, field{key: fld.key, value: f.apply(fld.value)})
		}
		return out
	case array:
		out := make(array, 0, len(v))
		for _, elem := range v {
			if f.drop("", elem) {
				continue
			}
			out = append(out, f.apply(elem))
		}
		return out
	}
	return v
}

// isBinary recognises the {length = N, bytes = 0x...} blobs `defaults` prints.
func isBinary(v value) bool {
	d, ok := v.(dict)
	if !ok || len(d) != 2 {
		return false
	}
	for _, fld := range d {
		switch fld.key {
		case "length":
			// 0 and 1 byte blobs arrive as booleans.
			switch fld.value.(type) {
			case num, boolean:
			default:
				return false
			}
		case "bytes":
			bytes, ok := fld.value.(str)
			if !ok || !strings.HasPrefix(string(bytes), "0x") {
				return false
			}
		default:
			return false
		}
	}
	return true
}

var dateLayouts = []string{
	"2006-01-02 15:04:05 -0700",
	"2006-01-02 15:04:05",
	"2006-01-02T15:04:05Z07:00",
	"2006-01-02T15:04:05",
	"2006-01-02",
}

func isDateString(s string) bool {
	for _, layout := range dateLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return inRange(t)
		}
	}
	// `defaults` also prints partial timestamps, e.g. 2026-08-20T15.
	if len(s) > 10 && (s[10] == 'T' || s[10] == ' ') {
		if t, err := time.Parse("2006-01-02", s[:10]); err == nil {
			return inRange(t)
		}
	}
	return false
}

func inRange(t time.Time) bool { return t.Year() >= 1900 && t.Year() <= 2100 }

var (
	uuidPattern   = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	hashedPattern = regexp.MustCompile(`^_[0-9a-fA-F]{32}$`)
)

func isUUID(s string) bool      { return uuidPattern.FindString(s) == s }
func isHashedID(s string) bool  { return hashedPattern.MatchString(s) }
func isUUIDKey(key string) bool { return uuidPattern.MatchString(key) }

// timestampWords match keys whose value is a time rather than a setting.
var timestampWords = []string{
	"time", "timestamp", "date", "epoch",
	"updated", "created", "modified", "changed",
	"lastused", "lastseen", "lastaccess", "lastconnected",
	"lastunseen", "lastvisit", "lastopen", "lastlaunch",
	"accessed", "visited", "opened", "launched",
	"expiry", "expires", "expired", "expiration",
	"checkedat", "setat", "startedat", "endedat",
	"since", "until", "when",
}

func isTimestampKey(key string) bool {
	lower := strings.ToLower(key)
	for _, word := range timestampWords {
		if strings.Contains(lower, word) {
			return true
		}
	}
	// lastConnected@Display:2, appDisconnected@Display:3
	if strings.Contains(key, "@") &&
		(strings.Contains(lower, "connected") || strings.Contains(lower, "seen") || strings.Contains(lower, "accessed")) {
		return true
	}
	// checkedAt, setAt, startedAt...
	return strings.HasSuffix(key, "At")
}

// uiStateWords match keys that describe window and toolbar layout.
var uiStateWords = []string{
	"NSWindow Frame ",
	"NSSplitView Subview Frames ",
	"NSNavPanelExpandedSize",
	"NSNavPanelFileLastListMode",
	"NSNavPanelFileListMode",
	"NSTableView Columns ",
	"NSTableView Sort Ordering ",
	"NSTableView Supports ",
	"Column Width",
	"UserColumnSortPerTab",
	"UserColumnsPerTab",
	"TB Icon Size Mode",
	"TB Size Mode",
	"image window frame",
	"image window parent frame",
	"NSPreferencesContentSize",
	"NSToolbar Configuration",
	"ExtensionsToolbarConfiguration",
	"CropRect",
}

func isUIStateKey(key string) bool {
	for _, word := range uiStateWords {
		if strings.Contains(key, word) {
			return true
		}
	}
	if strings.Contains(key, "cache") || strings.Contains(key, "Cache") {
		return true
	}
	return strings.HasSuffix(key, "Frame") &&
		(strings.Contains(key, "Window") || strings.Contains(key, "window"))
}

func isUIStateValue(v string) bool {
	// NSRect: {{x, y}, {width, height}}
	if strings.HasPrefix(v, "{{") && strings.HasSuffix(v, "}}") {
		return true
	}
	// NSSize: {width, height}
	if strings.HasPrefix(v, "{") && strings.HasSuffix(v, "}") &&
		strings.Count(v, ",") == 1 && !strings.Contains(v, "=") {
		return true
	}
	// Window frame: eight numbers
	fields := strings.Fields(v)
	if len(fields) == 8 {
		allNumbers := true
		for _, field := range fields {
			if _, err := strconv.ParseFloat(field, 64); err != nil {
				allNumbers = false
				break
			}
		}
		if allNumbers {
			return true
		}
	}
	// Split view frame: six comma-separated values ending in NO or YES
	if strings.Count(v, ",") == 5 {
		trimmed := strings.TrimSpace(v)
		if strings.HasSuffix(trimmed, "NO") || strings.HasSuffix(trimmed, "YES") {
			return true
		}
	}
	return false
}
