package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
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
	if len(a.Values) == 0 {
		return "[]"
	}

	indentStr := strings.Repeat("  ", indent)
	nextIndentStr := strings.Repeat("  ", indent+1)

	var parts []string
	parts = append(parts, "[")

	for _, v := range a.Values {
		parts = append(parts, nextIndentStr+v.ToNix(indent+1))
	}

	parts = append(parts, indentStr+"]")
	return strings.Join(parts, "\n")
}

type DictValue struct {
	Values map[string]Value
	Order  []string // Preserve order
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

func parseValue(input string) Value {
	input = strings.TrimSpace(input)

	// Handle arrays (parentheses)
	if strings.HasPrefix(input, "(") && strings.HasSuffix(input, ")") {
		return parseArray(input)
	}

	// Handle dictionaries (braces)
	if strings.HasPrefix(input, "{") && strings.HasSuffix(input, "}") {
		// Check if this is a data value - skip binary data as it's not useful in Nix
		if strings.Contains(input, "length =") && strings.Contains(input, "bytes =") {
			// Count semicolons to distinguish between data values and dicts with data properties
			semicolonCount := strings.Count(input, ";")
			equalsCount := strings.Count(input, " = ")

			// Data values typically have exactly 2 key=value pairs (length and bytes)
			if equalsCount == 2 && semicolonCount <= 2 {
				// Return SkipValue to indicate this should be skipped
				return SkipValue{}
			}
		}
		return parseDict(input)
	}

	// Handle quoted strings - remove quotes and unescape
	if strings.HasPrefix(input, "\"") && strings.HasSuffix(input, "\"") && len(input) > 1 {
		unescaped := input[1 : len(input)-1]
		// Unescape the string content
		unescaped = strings.ReplaceAll(unescaped, "\\\"", "\"")
		unescaped = strings.ReplaceAll(unescaped, "\\\\", "\\")
		return StringValue{Value: unescaped}
	}

	// Everything else is a string value
	return StringValue{Value: input}
}

func parseArray(input string) ArrayValue {
	content := input[1 : len(input)-1] // Remove outer parentheses
	content = strings.TrimSpace(content)

	if content == "" {
		return ArrayValue{Values: []Value{}}
	}

	values := parseArrayElements(content)
	return ArrayValue{Values: values}
}

func parseArrayElements(content string) []Value {
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
					values = append(values, parseValue(val))
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
		values = append(values, parseValue(val))
	}

	return values
}

func parseDict(input string) DictValue {
	content := input[1 : len(input)-1] // Remove outer braces
	content = strings.TrimSpace(content)

	if content == "" {
		return DictValue{Values: make(map[string]Value), Order: []string{}}
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
					values[currentKey] = parseValue(valueStr)
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
		values[currentKey] = parseValue(valueStr)
		order = append(order, currentKey)
	}

	return DictValue{Values: values, Order: order}
}

func convertDefaults(input io.Reader) (string, error) {
	scanner := bufio.NewScanner(input)
	var content strings.Builder

	for scanner.Scan() {
		content.WriteString(scanner.Text() + "\n")
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	inputStr := strings.TrimSpace(content.String())
	value := parseValue(inputStr)
	return value.ToNix(0), nil
}

func convertDefaultsWithValue(inputStr string) (string, Value, error) {
	value := parseValue(inputStr)
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
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [flags] [domain]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "A tool for converting macOS defaults into Nix templates.\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nArguments:\n")
		fmt.Fprintf(os.Stderr, "  domain\n")
		fmt.Fprintf(os.Stderr, "	The domain to convert (e.g., com.apple.dock).\n")
		fmt.Fprintf(os.Stderr, "  -out, -o <path>\n")
		fmt.Fprintf(os.Stderr, "	Output file or directory path. Mandatory with -split, optional otherwise.\n")
	}

	all := flag.Bool("all", false, "Process all defaults from `defaults read`")

	split := flag.Bool("split", false, "Split defaults into individual Nix files by domain")
	out := flag.String("out", "", "Output file or directory path")
	flag.Parse()

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

		result, err := convertDefaults(strings.NewReader(string(output)))
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
		for _, domain := range domains {
			domain = strings.TrimSpace(domain)
			if domain == "" {
				continue
			}

			// Read defaults for the domain
			readCmd := exec.Command("defaults", "read", domain)
			domainOutput, err := readCmd.Output()
			if err != nil {
				// Silently skip domains that can't be read
				continue
			}

			// Convert to Nix
			nixResult, err := convertDefaults(strings.NewReader(string(domainOutput)))
			if err != nil {
				continue
			}

			// Skip empty results
			if strings.TrimSpace(nixResult) == "{}" || strings.TrimSpace(nixResult) == "" {
				continue
			}

			// Write to file
			filename := filepath.Join(*out, fmt.Sprintf("%s.nix", domain))
			err = os.WriteFile(filename, []byte(nixResult), 0644)
			if err != nil {
				continue
			}

			successCount++
		}

		if successCount == 0 {
			fmt.Fprintf(os.Stderr, "Warning: No domains could be processed successfully.\n")
		}
	} else if len(flag.Args()) > 0 {
		domain := flag.Args()[0]
		cmd := exec.Command("defaults", "read", domain)
		output, err := cmd.Output()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error executing 'defaults read %s': %v\n", domain, err)
			os.Exit(1)
		}

		result, err := convertDefaults(strings.NewReader(string(output)))
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
