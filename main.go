package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
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
		for _, keyword := range nixKeywords {
			if key == keyword {
				needsQuoting = true
				break
			}
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

func main() {
	var input io.Reader = os.Stdin

	if len(os.Args) > 1 {
		file, err := os.Open(os.Args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening file: %v\n", err)
			os.Exit(1)
		}
		defer file.Close()
		input = file
	}

	result, err := convertDefaults(input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error converting defaults: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(result)
}
