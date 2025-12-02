package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
)

type Value interface {
	ToNix(indent int) string
}

type SkipValue struct{}

func (s SkipValue) ToNix(indent int) string {
	return ""
}

type StringValue struct {
	Value string
}

func (s StringValue) ToNix(indent int) string {
	// Handle special boolean cases
	if s.Value == "1" {
		return "true"
	}
	if s.Value == "0" {
		return "false"
	}

	// Handle numeric values
	if num, err := strconv.Atoi(s.Value); err == nil {
		return strconv.Itoa(num)
	}
	if num, err := strconv.ParseFloat(s.Value, 64); err == nil {
		return fmt.Sprintf("%.15g", num)
	}

	// Handle unquoted identifiers that should remain as strings
	if !strings.Contains(s.Value, " ") && !strings.Contains(s.Value, "/") &&
		!strings.Contains(s.Value, ".") && !strings.Contains(s.Value, ":") &&
		len(s.Value) > 0 && s.Value != "true" && s.Value != "false" {
		return fmt.Sprintf("\"%s\"", s.Value)
	}

	// Escape and quote strings
	escaped := strings.ReplaceAll(s.Value, "\\", "\\\\")
	escaped = strings.ReplaceAll(escaped, "\"", "\\\"")
	// Escape Nix string interpolation syntax ${...} → $''{...}
	escaped = strings.ReplaceAll(escaped, "${", "$''{")
	return fmt.Sprintf("\"%s\"", escaped)
}

type ArrayValue struct {
	Values []Value
}

func (a ArrayValue) ToNix(indent int) string {
	// Filter out SkipValue entries
	var validValues []Value
	for _, v := range a.Values {
		if _, isSkip := v.(SkipValue); !isSkip {
			validValues = append(validValues, v)
		}
	}

	if len(validValues) == 0 {
		return "[]"
	}

	indentStr := strings.Repeat("  ", indent)
	nextIndentStr := strings.Repeat("  ", indent+1)

	var parts []string
	parts = append(parts, "[")

	for _, v := range validValues {
		parts = append(parts, nextIndentStr+v.ToNix(indent+1))
	}

	parts = append(parts, indentStr+"]")
	return strings.Join(parts, "\n")
}

type DictValue struct {
	Values map[string]Value
	Order  []string // Preserve order
	config ParseConfig // Add config for filtering
}

func (d DictValue) ToNix(indent int) string {
	if len(d.Values) == 0 {
		return "{}"
	}

	indentStr := strings.Repeat("  ", indent)
	nextIndentStr := strings.Repeat("  ", indent+1)

	var parts []string
	parts = append(parts, "{")

	keys := d.Order
	if len(keys) == 0 {
		// Fall back to map iteration if no order preserved
		for k := range d.Values {
			keys = append(keys, k)
		}
	}

	for _, key := range keys {
		value, exists := d.Values[key]
		if !exists {
			continue
		}

		// Skip binary data values
		if _, isSkip := value.(SkipValue); isSkip {
			continue
		}

		// Skip UI state keys if filtering is enabled
		if d.config.NoState && isUIStateKey(key) {
			continue
		}

		// Skip UUID keys if filtering is enabled
		if d.config.NoUUIDs && isUUIDKey(key) {
			continue
		}

		// Skip timestamp keys if date filtering is enabled
		if d.config.NoDates && isTimestampKey(key) {
			// Also check if the value looks like a timestamp
			if sv, ok := value.(StringValue); ok {
				// Check if it's a numeric timestamp
				if num, err := strconv.ParseFloat(sv.Value, 64); err == nil {
					if isUnixTimestamp(num) || isCFAbsoluteTime(num) {
						continue
					}
				}
			}
			// For non-numeric values with timestamp keys, still skip them
			continue
		}

		nixKey := key
		// Quote keys that need it
		needsQuoting := false

		// Check if key is purely numeric
		if _, err := strconv.Atoi(key); err == nil {
			needsQuoting = true
		}

		// Check if key starts with a number
		if len(key) > 0 && key[0] >= '0' && key[0] <= '9' {
			needsQuoting = true
		}

		// Check if key is a Nix reserved keyword
		nixKeywords := []string{
			"with", "let", "in", "if", "then", "else", "assert", "rec",
			"inherit", "or", "and", "import", "builtins", "throw", "abort",
			"true", "false", "null",
		}
		if slices.Contains(nixKeywords, key) {
			needsQuoting = true
		}

		// Check for other characters that need quoting
		if strings.Contains(key, " ") || strings.Contains(key, "-") ||
			strings.Contains(key, ".") || strings.HasPrefix(key, "\"") {
			needsQuoting = true
		}

		if needsQuoting && !strings.HasPrefix(key, "\"") {
			nixKey = fmt.Sprintf("\"%s\"", strings.ReplaceAll(key, "\"", "\\\""))
		}

		valueStr := value.ToNix(indent + 1)
		if strings.Contains(valueStr, "\n") {
			// Add proper indentation to multiline values
			parts = append(parts, fmt.Sprintf("%s%s = %s;", nextIndentStr, nixKey, valueStr))
		} else {
			parts = append(parts, fmt.Sprintf("%s%s = %s;", nextIndentStr, nixKey, valueStr))
		}
	}

	parts = append(parts, indentStr+"}")
	return strings.Join(parts, "\n")
}

type ParseConfig struct {
	NoDates bool
	NoState bool
	NoUUIDs bool
}

func isBinaryDataValue(input string) bool {
	// More robust binary data detection
	// Binary data values can have the patterns:
	// {length = N, bytes = 0x...} (comma-separated)
	// {length = N; bytes = 0x...;} (semicolon-separated)
	
	// Must contain both "length =" and "bytes =" 
	if !strings.Contains(input, "length =") || !strings.Contains(input, "bytes =") {
		return false
	}
	
	// Check for the specific hex bytes pattern
	if !strings.Contains(input, "bytes = 0x") {
		return false
	}
	
	// Parse the content to ensure it only contains length and bytes keys
	content := strings.TrimSpace(input[1 : len(input)-1]) // Remove braces
	
	// Try both comma and semicolon separators
	var parts []string
	if strings.Contains(content, ";") {
		parts = strings.Split(content, ";")
	} else {
		parts = strings.Split(content, ",")
	}
	
	validKeys := 0
	
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		
		if strings.HasPrefix(part, "length =") || strings.HasPrefix(part, "bytes = 0x") {
			validKeys++
		} else {
			// Found a key that's not length or bytes, so this isn't binary data
			return false
		}
	}
	
	// Should have exactly 2 valid keys (length and bytes)
	return validKeys == 2
}

func isUIStateKey(key string) bool {
	// UI state and window geometry that's typically not useful for Nix config
	statePatterns := []string{
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
	}
	
	for _, pattern := range statePatterns {
		if strings.Contains(key, pattern) {
			return true
		}
	}
	
	// NSToolbar configurations - these are UI state
	if strings.Contains(key, "NSToolbar Configuration") ||
		strings.Contains(key, "ExtensionsToolbarConfiguration") {
		return true
	}
	
	// Crop rectangles and other UI geometry (but be more specific)
	if strings.Contains(key, "CropRect") {
		return true
	}
	
	// Window frames that don't start with NSWindow Frame
	if strings.HasSuffix(key, "Frame") && 
		(strings.Contains(key, "Window") || strings.Contains(key, "window")) {
		return true
	}
	
	// Cache and temporary data
	if strings.Contains(key, "cache") || strings.Contains(key, "Cache") {
		return true
	}
	
	return false
}

func isUIStateValue(value string) bool {
	// NSRect format: {{x, y}, {width, height}}
	if strings.HasPrefix(value, "{{") && strings.HasSuffix(value, "}}") {
		return true
	}
	
	// NSSize format: {width, height}  
	if strings.HasPrefix(value, "{") && strings.HasSuffix(value, "}") && 
		strings.Count(value, ",") == 1 && !strings.Contains(value, "=") {
		return true
	}
	
	// Window frame format: 8 space-separated numbers
	parts := strings.Fields(value)
	if len(parts) == 8 {
		allNumbers := true
		for _, part := range parts {
			if _, err := strconv.ParseFloat(part, 64); err != nil {
				allNumbers = false
				break
			}
		}
		if allNumbers {
			return true
		}
	}
	
	// Split view frame format: 6 comma-separated values ending with NO/YES
	if strings.Count(value, ",") == 5 && 
		(strings.HasSuffix(strings.TrimSpace(value), "NO") || 
		 strings.HasSuffix(strings.TrimSpace(value), "YES")) {
		return true
	}
	
	return false
}

func isDateString(s string) bool {
	// Common date patterns in macOS defaults
	// Simple heuristic: check for YYYY-MM-DD pattern
	if len(s) < 10 {
		return false
	}

	// Check for date patterns
	// Standard macOS format: 2025-06-07 12:01:44 +0000
	// ISO 8601: 2025-06-07T12:01:44Z
	// Date only: 2025-06-07

	// Must contain at least YYYY-MM-DD pattern
	if len(s) >= 10 && s[4] == '-' && s[7] == '-' {
		// Check if first 4 chars are digits (year) and validate range
		year := 0
		for i := 0; i < 4; i++ {
			if s[i] < '0' || s[i] > '9' {
				return false
			}
			year = year*10 + int(s[i]-'0')
		}
		if year < 1900 || year > 2100 {
			return false
		}
		
		// Check if chars 5-6 are digits and form valid month (01-12)
		if s[5] < '0' || s[5] > '9' || s[6] < '0' || s[6] > '9' {
			return false
		}
		month := int(s[5]-'0')*10 + int(s[6]-'0')
		if month < 1 || month > 12 {
			return false
		}
		
		// Check if chars 8-9 are digits and form valid day (01-31)
		if s[8] < '0' || s[8] > '9' || s[9] < '0' || s[9] > '9' {
			return false
		}
		day := int(s[8]-'0')*10 + int(s[9]-'0')
		if day < 1 || day > 31 {
			return false
		}
		
		// If we have exactly 10 chars, it's a date-only format
		if len(s) == 10 {
			return true
		}
		
		// For longer strings, check if char 10 is a separator (space or 'T')
		if len(s) > 10 && (s[10] == ' ' || s[10] == 'T') {
			// Additional validation for time portion if present
			if s[10] == ' ' && len(s) >= 19 {
				// Check HH:MM:SS format at positions 11-18
				timepart := s[11:19]
				if len(timepart) == 8 && timepart[2] == ':' && timepart[5] == ':' {
					// Validate time digits
					for _, pos := range []int{0, 1, 3, 4, 6, 7} {
						if timepart[pos] < '0' || timepart[pos] > '9' {
							return false
						}
					}
					hours := int(timepart[0]-'0')*10 + int(timepart[1]-'0')
					minutes := int(timepart[3]-'0')*10 + int(timepart[4]-'0')
					seconds := int(timepart[6]-'0')*10 + int(timepart[7]-'0')
					if hours > 23 || minutes > 59 || seconds > 59 {
						return false
					}
				}
			}
			return true
		}
	}

	return false
}

func isUUIDString(s string) bool {
	// UUID v4 format: XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX
	// where X is a hexadecimal digit
	if len(s) != 36 {
		return false
	}
	
	// Check hyphens at expected positions
	if s[8] != '-' || s[13] != '-' || s[18] != '-' || s[23] != '-' {
		return false
	}
	
	// Check that all other characters are hex digits
	for i, c := range s {
		// Skip hyphen positions
		if i == 8 || i == 13 || i == 18 || i == 23 {
			continue
		}
		
		// Must be a hex digit (0-9, a-f, A-F)
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	
	return true
}

func isHashedIDString(s string) bool {
	// Check for underscore-prefixed hex identifiers like "_19a3bc4999bddb89e1a44f4b87bdc37c"
	// These appear to be 32-character hex strings (possibly MD5 hashes)
	if len(s) < 2 || s[0] != '_' {
		return false
	}
	
	// Check if the rest is a 32-character hex string
	hexPart := s[1:]
	if len(hexPart) != 32 {
		return false
	}
	
	// Check that all characters are hex digits
	for _, c := range hexPart {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	
	return true
}

func isUUIDKey(key string) bool {
	// Check if the key itself is a UUID
	if isUUIDString(key) {
		return true
	}
	
	// Check if the key contains a UUID (common pattern: prefix-UUID-suffix)
	if len(key) >= 36 {
		// Look for UUID pattern within the key
		for i := 0; i <= len(key)-36; i++ {
			if isUUIDString(key[i:i+36]) {
				return true
			}
		}
	}
	
	return false
}

func isTimestampKey(key string) bool {
	// Convert key to lowercase for case-insensitive matching
	lowerKey := strings.ToLower(key)
	
	// Common timestamp-related patterns in keys
	timestampPatterns := []string{
		"time", "timestamp", "date", "epoch",
		"updated", "created", "modified", "changed",
		"lastused", "lastseen", "lastaccess", "lastconnected",
		"lastunseen", "lastvisit", "lastopen", "lastlaunch",
		"accessed", "visited", "opened", "launched",
		"expiry", "expires", "expired", "expiration",
		"checkedat", "setat", "startedat", "endedat",
		"since", "until", "when", "at",
	}
	
	// Check if the key contains any timestamp-related pattern
	for _, pattern := range timestampPatterns {
		if strings.Contains(lowerKey, pattern) {
			return true
		}
	}
	
	// Check for patterns like "connected@", "seen@" etc.
	if strings.Contains(key, "@") && (strings.Contains(lowerKey, "connected") || 
		strings.Contains(lowerKey, "seen") || strings.Contains(lowerKey, "accessed")) {
		return true
	}
	
	return false
}

func isUnixTimestamp(value float64) bool {
	// Unix timestamps for dates between 2000 and 2040
	// 2000-01-01: 946684800
	// 2040-01-01: 2208988800
	return value >= 946684800 && value <= 2208988800
}

func isCFAbsoluteTime(value float64) bool {
	// CFAbsoluteTime for dates between 2000 and 2040
	// Seconds since 2001-01-01
	// 2000-01-01 from 2001-01-01: -31536000 (negative)
	// 2040-01-01 from 2001-01-01: 1230768000
	// Also require a minimum value to avoid small numbers
	return value >= 100000000 && value <= 1230768000
}

func parseValue(input string) Value {
	return parseValueWithConfig(input, ParseConfig{})
}

func parseValueWithConfig(input string, config ParseConfig) Value {
	input = strings.TrimSpace(input)

	// Handle arrays (parentheses)
	if strings.HasPrefix(input, "(") && strings.HasSuffix(input, ")") {
		return parseArrayWithConfig(input, config)
	}

	// Handle dictionaries (braces)
	if strings.HasPrefix(input, "{") && strings.HasSuffix(input, "}") {
		// Check if this is a binary data value - skip binary data as it's not useful in Nix
		if isBinaryDataValue(input) {
			return SkipValue{}
		}
		dictValue := parseDictWithConfig(input, config)
		dictValue.config = config // Ensure config is set
		return dictValue
	}

	// Handle quoted strings - remove quotes and unescape
	if strings.HasPrefix(input, "\"") && strings.HasSuffix(input, "\"") && len(input) > 1 {
		unescaped := input[1 : len(input)-1]
		// Unescape the string content
		unescaped = strings.ReplaceAll(unescaped, "\\\"", "\"")
		unescaped = strings.ReplaceAll(unescaped, "\\\\", "\\")

		// Check if this is a date and should be skipped
		if config.NoDates && isDateString(unescaped) {
			return SkipValue{}
		}

		// Check if this is UI state and should be skipped
		if config.NoState && isUIStateValue(unescaped) {
			return SkipValue{}
		}

		// Check if this is a UUID and should be skipped
		if config.NoUUIDs && (isUUIDString(unescaped) || isHashedIDString(unescaped)) {
			return SkipValue{}
		}

		return StringValue{Value: unescaped}
	}

	// Everything else is a string value
	// Check if this is a date and should be skipped
	if config.NoDates && isDateString(input) {
		return SkipValue{}
	}

	// Check if this is UI state and should be skipped
	if config.NoState && isUIStateValue(input) {
		return SkipValue{}
	}

	// Check if this is a UUID and should be skipped
	if config.NoUUIDs && (isUUIDString(input) || isHashedIDString(input)) {
		return SkipValue{}
	}

	return StringValue{Value: input}
}

func parseArray(input string) ArrayValue {
	return parseArrayWithConfig(input, ParseConfig{})
}

func parseArrayWithConfig(input string, config ParseConfig) ArrayValue {
	content := input[1 : len(input)-1] // Remove outer parentheses
	content = strings.TrimSpace(content)

	if content == "" {
		return ArrayValue{Values: []Value{}}
	}

	values := parseArrayElementsWithConfig(content, config)
	return ArrayValue{Values: values}
}

func parseArrayElements(content string) []Value {
	return parseArrayElementsWithConfig(content, ParseConfig{})
}

func parseArrayElementsWithConfig(content string, config ParseConfig) []Value {
	var values []Value
	var current strings.Builder
	var depth int
	var inQuotes bool
	var escape bool

	runes := []rune(content)
	for i := range runes {
		char := runes[i]

		if escape {
			current.WriteRune(char)
			escape = false
			continue
		}

		if char == '\\' {
			escape = true
			current.WriteRune(char)
			continue
		}

		if char == '"' {
			inQuotes = !inQuotes
			current.WriteRune(char)
			continue
		}

		if !inQuotes {
			switch char {
			case '(', '{':
				depth++
			case ')', '}':
				depth--
			}

			if char == ',' && depth == 0 {
				val := strings.TrimSpace(current.String())
				val = strings.TrimSuffix(val, ";")
				if val != "" {
					values = append(values, parseValueWithConfig(val, config))
				}
				current.Reset()
				continue
			}
		}

		current.WriteRune(char)
	}

	// Handle the last element
	val := strings.TrimSpace(current.String())
	val = strings.TrimSuffix(val, ";")
	if val != "" {
		values = append(values, parseValueWithConfig(val, config))
	}

	return values
}

func parseDict(input string) DictValue {
	return parseDictWithConfig(input, ParseConfig{})
}

func parseDictWithConfig(input string, config ParseConfig) DictValue {
	content := input[1 : len(input)-1] // Remove outer braces
	content = strings.TrimSpace(content)

	if content == "" {
		return DictValue{Values: make(map[string]Value), Order: []string{}, config: config}
	}

	values := make(map[string]Value)
	var order []string

	// Parse using a character-by-character approach to handle nested structures
	var currentKey string
	var currentValue strings.Builder
	var inKey = true
	var depth int
	var inQuotes bool
	var escape bool

	runes := []rune(content)
	i := 0

	for i < len(runes) {
		char := runes[i]

		if escape {
			currentValue.WriteRune(char)
			escape = false
			i++
			continue
		}

		if char == '\\' {
			escape = true
			currentValue.WriteRune(char)
			i++
			continue
		}

		if char == '"' {
			inQuotes = !inQuotes
			if inKey {
				currentKey += string(char)
			} else {
				currentValue.WriteRune(char)
			}
			i++
			continue
		}

		if !inQuotes {
			if inKey {
				if char == '=' && i+2 < len(runes) && runes[i+1] == ' ' {
					// Found key = value separator
					currentKey = strings.TrimSpace(currentKey)
					inKey = false
					i += 2 // Skip " = "
					continue
				} else {
					currentKey += string(char)
				}
			} else {
				// In value
				switch char {
				case '{', '(':
					depth++
				case '}', ')':
					depth--
				}

				if char == ';' && depth == 0 {
					// End of value
					valueStr := strings.TrimSpace(currentValue.String())
					values[currentKey] = parseValueWithConfig(valueStr, config)
					order = append(order, currentKey)

					// Reset for next key-value pair
					currentKey = ""
					currentValue.Reset()
					inKey = true

					// Skip whitespace after semicolon
					i++
					for i < len(runes) && (runes[i] == ' ' || runes[i] == '\t' || runes[i] == '\n' || runes[i] == '\r') {
						i++
					}
					continue
				} else {
					currentValue.WriteRune(char)
				}
			}
		} else {
			// Inside quotes
			if inKey {
				currentKey += string(char)
			} else {
				currentValue.WriteRune(char)
			}
		}

		i++
	}

	// Handle the last key-value pair if it doesn't end with semicolon
	if currentKey != "" && currentValue.Len() > 0 {
		valueStr := strings.TrimSpace(currentValue.String())
		values[currentKey] = parseValueWithConfig(valueStr, config)
		order = append(order, currentKey)
	}

	return DictValue{Values: values, Order: order, config: config}
}

func convertDefaults(input io.Reader) (string, error) {
	return convertDefaultsWithConfig(input, ParseConfig{})
}

func convertDefaultsWithConfig(input io.Reader, config ParseConfig) (string, error) {
	scanner := bufio.NewScanner(input)
	var content strings.Builder

	for scanner.Scan() {
		content.WriteString(scanner.Text() + "\n")
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	inputStr := strings.TrimSpace(content.String())
	value := parseValueWithConfig(inputStr, config)
	return value.ToNix(0), nil
}

func convertDefaultsWithValue(inputStr string) (string, Value, error) {
	return convertDefaultsWithValueAndConfig(inputStr, ParseConfig{})
}

func convertDefaultsWithValueAndConfig(inputStr string, config ParseConfig) (string, Value, error) {
	value := parseValueWithConfig(inputStr, config)
	return value.ToNix(0), value, nil
}

func extractBundleIDs(value Value) map[string]Value {
	bundleMap := make(map[string]Value)

	if dict, ok := value.(DictValue); ok {
		for key, val := range dict.Values {
			// Skip binary data values
			if _, isSkip := val.(SkipValue); isSkip {
				continue
			}
			// Include all top-level keys - bundle IDs, NSGlobalDomain, and custom preferences
			bundleMap[key] = val
		}
	}

	return bundleMap
}

func sanitizeFilename(key string) string {
	// Remove quotes if present
	filename := strings.Trim(key, "\"")
	// Replace dots with hyphens for filename safety
	filename = strings.ReplaceAll(filename, ".", "-")
	// Replace any other problematic characters
	filename = strings.ReplaceAll(filename, " ", "_")
	filename = strings.ReplaceAll(filename, "/", "_")
	return filename
}

func main() {
	// Check if running on macOS
	if runtime.GOOS != "darwin" {
		fmt.Fprintf(os.Stderr, "Error: defaults2nix is designed for macOS only (requires 'defaults' command).\n")
		fmt.Fprintf(os.Stderr, "Current platform: %s\n", runtime.GOOS)
		os.Exit(1)
	}

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [flags] [domain]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "A tool for converting macOS defaults into Nix templates.\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nArguments:\n")
		fmt.Fprintf(os.Stderr, "  domain\n")
		fmt.Fprintf(os.Stderr, "	The domain to convert (e.g., com.apple.dock).\n")
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  defaults2nix com.apple.Safari\n")
		fmt.Fprintf(os.Stderr, "  defaults2nix com.apple.Safari -o safari.nix\n")
		fmt.Fprintf(os.Stderr, "  defaults2nix -all -o all-defaults.nix\n")
		fmt.Fprintf(os.Stderr, "  defaults2nix -all -filter dates -o all-defaults.nix\n")
		fmt.Fprintf(os.Stderr, "  defaults2nix -all -filter state,uuids -o all-defaults.nix\n")
		fmt.Fprintf(os.Stderr, "  defaults2nix -all -filter dates,state,uuids -o all-defaults.nix\n")
		fmt.Fprintf(os.Stderr, "  defaults2nix -split -o ./configs/\n")
		fmt.Fprintf(os.Stderr, "  sudo defaults2nix -all -o all-defaults.nix  # for system configs\n")
	}

	all := flag.Bool("all", false, "Process all defaults from `defaults read`")
	filter := flag.String("filter", "", "Comma-separated list of items to filter out (dates,state,uuids)")
	split := flag.Bool("split", false, "Split defaults into individual Nix files by domain")
	out := flag.String("out", "", "Output file or directory path")
	flag.Parse()
	
	// Parse filter options
	var noDates, noState, noUUIDs bool
	if *filter != "" {
		filters := strings.Split(*filter, ",")
		for _, f := range filters {
			switch strings.TrimSpace(strings.ToLower(f)) {
			case "dates":
				noDates = true
			case "state":
				noState = true
			case "uuids":
				noUUIDs = true
			default:
				fmt.Fprintf(os.Stderr, "Error: Unknown filter option '%s'. Valid options are: dates, state, uuids\n", f)
				os.Exit(1)
			}
		}
	}

	// No flags and no args, show usage
	if !*all && !*split && *out == "" && len(flag.Args()) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	// Prevent using flags with domain argument
	if (*all || *split) && len(flag.Args()) > 0 {
		fmt.Fprintf(os.Stderr, "Error: Cannot use -all or -split with a domain argument.\n")
		flag.Usage()
		os.Exit(1)
	}

	// Prevent using -all and -split together
	if *all && *split {
		fmt.Fprintf(os.Stderr, "Error: Cannot use -all and -split at the same time.\n")
		flag.Usage()
		os.Exit(1)
	}

	// Handle -out flag based on -split
	if *split {
		if *out == "" {
			fmt.Fprintf(os.Stderr, "Error: -out is mandatory when -split is used.\n")
			flag.Usage()
			os.Exit(1)
		}
		fileInfo, err := os.Stat(*out)
		if os.IsNotExist(err) {
			// Try to create the directory if it doesn't exist
			err = os.MkdirAll(*out, 0755)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating output directory %s: %v\n", *out, err)
				os.Exit(1)
			}
		} else if err != nil {
			fmt.Fprintf(os.Stderr, "Error checking output path %s: %v\n", *out, err)
			os.Exit(1)
		} else if !fileInfo.IsDir() {
			fmt.Fprintf(os.Stderr, "Error: -out path %s must be a directory when -split is used.\n", *out)
			flag.Usage()
			os.Exit(1)
		}
	} else if *out != "" && (*all || len(flag.Args()) > 0) {
		// If -out is provided without -split, it must be a file
		fileInfo, err := os.Stat(*out)
		if err == nil && fileInfo.IsDir() {
			fmt.Fprintf(os.Stderr, "Error: -out path %s must be a file when not using -split.\n", *out)
			flag.Usage()
			os.Exit(1)
		}
	}

	if *all {
		cmd := exec.Command("defaults", "read")
		output, err := cmd.Output()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error executing 'defaults read': %v\n", err)
			os.Exit(1)
		}

		result, err := convertDefaultsWithConfig(strings.NewReader(string(output)), ParseConfig{NoDates: noDates, NoState: noState, NoUUIDs: noUUIDs})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error converting defaults: %v\n", err)
			os.Exit(1)
		}
		if *out != "" {
			err = os.WriteFile(*out, []byte(result), 0644)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error writing to file %s: %v\n", *out, err)
				os.Exit(1)
			}
		} else {
			fmt.Println(result)
		}
	} else if *split {
		cmd := exec.Command("defaults", "domains")
		output, err := cmd.Output()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error executing 'defaults domains': %v\n", err)
			os.Exit(1)
		}

		domains := strings.Split(string(output), ", ")
		successCount := 0
		var skippedDomains []string
		var errorDomains []string
		
		for _, domain := range domains {
			domain = strings.TrimSpace(domain)
			if domain == "" {
				continue
			}

			// Read defaults for the domain
			readCmd := exec.Command("defaults", "read", domain)
			domainOutput, err := readCmd.Output()
			if err != nil {
				errorDomains = append(errorDomains, domain)
				continue
			}

			// Convert to Nix
			nixResult, err := convertDefaultsWithConfig(strings.NewReader(string(domainOutput)), ParseConfig{NoDates: noDates, NoState: noState, NoUUIDs: noUUIDs})
			if err != nil {
				errorDomains = append(errorDomains, domain)
				continue
			}

			// Skip empty results
			if strings.TrimSpace(nixResult) == "{}" || strings.TrimSpace(nixResult) == "" {
				skippedDomains = append(skippedDomains, domain)
				continue
			}

			// Write to file
            filename := filepath.Join(*out, fmt.Sprintf("%s.nix", sanitizeFilename(domain)))
			err = os.WriteFile(filename, []byte(nixResult), 0644)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Failed to write %s: %v\n", filename, err)
				continue
			}

			successCount++
		}

		// Provide detailed feedback
		if successCount == 0 {
			fmt.Fprintf(os.Stderr, "Error: No domains could be processed successfully.\n")
			if len(errorDomains) > 0 {
				fmt.Fprintf(os.Stderr, "Domains with errors: %s\n", strings.Join(errorDomains, ", "))
			}
			os.Exit(1)
		} else {
			if len(skippedDomains) > 0 {
				fmt.Fprintf(os.Stderr, "Info: Skipped %d empty domains: %s\n", len(skippedDomains), strings.Join(skippedDomains, ", "))
			}
			if len(errorDomains) > 0 {
				fmt.Fprintf(os.Stderr, "Warning: Failed to process %d domains: %s\n", len(errorDomains), strings.Join(errorDomains, ", "))
			}
			fmt.Fprintf(os.Stderr, "Successfully processed %d domains to %s\n", successCount, *out)
		}
	} else if len(flag.Args()) > 0 {
		domain := flag.Args()[0]
		cmd := exec.Command("defaults", "read", domain)
		output, err := cmd.Output()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error executing 'defaults read %s': %v\n", domain, err)
			os.Exit(1)
		}

		result, err := convertDefaultsWithConfig(strings.NewReader(string(output)), ParseConfig{NoDates: noDates, NoState: noState, NoUUIDs: noUUIDs})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error converting defaults: %v\n", err)
			os.Exit(1)
		}
		if *out != "" {
			err = os.WriteFile(*out, []byte(result), 0644)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error writing to file %s: %v\n", *out, err)
				os.Exit(1)
			}
		} else {
			fmt.Println(result)
		}
	}
}
