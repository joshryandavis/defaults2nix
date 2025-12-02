package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

func TestStringValue_ToNix(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Boolean true", "1", "true"},
		{"Boolean false", "0", "false"},
		{"Integer", "42", "42"},
		{"Float", "3.14", "3.14"},
		{"Simple string", "hello", "\"hello\""},
		{"URL string", "https://www.apple.com/startpage/", "\"https://www.apple.com/startpage/\""},
		{"String with spaces", "hello world", "\"hello world\""},
		{"String with quotes", "say \"hello\"", "\"say \\\"hello\\\"\""},
		{"String with backslashes", "path\\\\to\\\\file", "\"path\\\\to\\\\file\""},
		{"Empty string", "", "\"\""},
		{"Date string", "2025-06-07 12:01:44 +0000", "\"2025-06-07 12:01:44 +0000\""},
		{"Identifier with dots", "com.example.app", "\"com.example.app\""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sv := StringValue{Value: tt.input}
			result := sv.ToNix(0)
			if result != tt.expected {
				t.Errorf("StringValue.ToNix() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestArrayValue_ToNix(t *testing.T) {
	tests := []struct {
		name     string
		values   []Value
		expected string
	}{
		{
			"Empty array",
			[]Value{},
			"[]",
		},
		{
			"Single string",
			[]Value{StringValue{Value: "hello"}},
			"[\n  \"hello\"\n]",
		},
		{
			"Multiple values",
			[]Value{
				StringValue{Value: "1"},
				StringValue{Value: "hello"},
				StringValue{Value: "https://example.com"},
			},
			"[\n  true\n  \"hello\"\n  \"https://example.com\"\n]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			av := ArrayValue{Values: tt.values}
			result := av.ToNix(0)
			if result != tt.expected {
				t.Errorf("ArrayValue.ToNix() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestDictValue_ToNix(t *testing.T) {
	tests := []struct {
		name     string
		values   map[string]Value
		order    []string
		expected string
	}{
		{
			"Empty dict",
			map[string]Value{},
			[]string{},
			"{}",
		},
		{
			"Simple dict",
			map[string]Value{
				"key1": StringValue{Value: "1"},
				"key2": StringValue{Value: "hello"},
			},
			[]string{"key1", "key2"},
			"{\n  key1 = true;\n  key2 = \"hello\";\n}",
		},
		{
			"Dict with quoted keys",
			map[string]Value{
				"0":          StringValue{Value: "numeric key"},
				"with-dash":  StringValue{Value: "dashed key"},
				"with space": StringValue{Value: "spaced key"},
			},
			[]string{"0", "with-dash", "with space"},
			"{\n  \"0\" = \"numeric key\";\n  \"with-dash\" = \"dashed key\";\n  \"with space\" = \"spaced key\";\n}",
		},
		{
			"Dict with skip values",
			map[string]Value{
				"key1": StringValue{Value: "hello"},
				"skip": SkipValue{},
				"key2": StringValue{Value: "world"},
			},
			[]string{"key1", "skip", "key2"},
			"{\n  key1 = \"hello\";\n  key2 = \"world\";\n}",
		},
		{
			"Nested dict",
			map[string]Value{
				"outer": DictValue{
					Values: map[string]Value{
						"inner": StringValue{Value: "nested"},
					},
					Order: []string{"inner"},
				},
			},
			[]string{"outer"},
			"{\n  outer = {\n    inner = \"nested\";\n  };\n}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dv := DictValue{Values: tt.values, Order: tt.order}
			result := dv.ToNix(0)
			if result != tt.expected {
				t.Errorf("DictValue.ToNix() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestParseValue(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected Value
	}{
		{"String value", "hello", StringValue{Value: "hello"}},
		{"Quoted string", "\"hello world\"", StringValue{Value: "hello world"}},
		{"Empty array", "()", ArrayValue{Values: []Value{}}},
		{"Array with values", "(hello, world)", ArrayValue{Values: []Value{
			StringValue{Value: "hello"},
			StringValue{Value: "world"},
		}}},
		{"Empty dict", "{}", DictValue{Values: map[string]Value{}, Order: []string{}}},
		{"Simple dict", "{key = value;}", DictValue{
			Values: map[string]Value{"key": StringValue{Value: "value"}},
			Order:  []string{"key"},
		}},
		{"Binary data", "{length = 256; bytes = 0x89504e47;}", SkipValue{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseValue(tt.input)
			if !compareValues(result, tt.expected) {
				t.Errorf("parseValue(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestConvertDefaults_SimpleTest(t *testing.T) {
	input := `{
    AllowJavaScriptFromAppleEvents = 1;
    AutoFillCreditCardData = 1;
    AutoOpenSafeDownloads = 0;
    ShowStandaloneTabBar = 0;
    HomePage = "https://www.apple.com/startpage/";
    ExtensionsEnabled = 1;
}`

	result, err := convertDefaults(strings.NewReader(input))
	if err != nil {
		t.Fatalf("convertDefaults() error = %v", err)
	}

	expected := `{
  AllowJavaScriptFromAppleEvents = true;
  AutoFillCreditCardData = true;
  AutoOpenSafeDownloads = false;
  ShowStandaloneTabBar = false;
  HomePage = "https://www.apple.com/startpage/";
  ExtensionsEnabled = true;
}`

	if result != expected {
		t.Errorf("convertDefaults() = %q, want %q", result, expected)
	}
}

func TestConvertDefaults_BinaryData(t *testing.T) {
	input := `{
    TestSetting = 1;
    HomePage = "https://example.com";
    BinaryData = {length = 256, bytes = 0x89504e47 0d0a1a0a 00000000 49484452};
    AnotherSetting = "value";
    MoreBinaryData = {length = 128, bytes = 0x12345678 abcdef90 deadbeef cafebabe};
    LastSetting = 0;
}`

	result, err := convertDefaults(strings.NewReader(input))
	if err != nil {
		t.Fatalf("convertDefaults() error = %v", err)
	}

	// Verify binary data is skipped completely
	if strings.Contains(result, "BinaryData") || strings.Contains(result, "MoreBinaryData") {
		t.Error("Binary data should be completely skipped from output")
	}

	// Verify other settings are preserved
	expectedContains := []string{
		"TestSetting = true;",
		"HomePage = \"https://example.com\";",
		"AnotherSetting = \"value\";",
		"LastSetting = false;",
	}

	for _, expected := range expectedContains {
		if !strings.Contains(result, expected) {
			t.Errorf("Expected result to contain: %s\nGot: %s", expected, result)
		}
	}
}

func TestConvertDefaults_ComplexSafariExample(t *testing.T) {
	input := `{
    AllowJavaScriptFromAppleEvents = 1;
    AutoFillCreditCardData = 1;
    FrequentlyVisitedSitesCache = (
        {
            Score = "33.52108001708984";
            Title = "(282) YouTube";
            URL = "https://www.youtube.com/";
        },
        {
            Score = "13.06611442565918";
            Title = LinkedIn;
            URL = "https://www.linkedin.com/";
        }
    );
    customizationSyncServerToken = {length = 293, bytes = 0x62706c69 73743030 d4010203 04050607};
    ShowStandaloneTabBar = 0;
}`

	result, err := convertDefaults(strings.NewReader(input))
	if err != nil {
		t.Fatalf("convertDefaults() error = %v", err)
	}

	// Test boolean conversions
	if !strings.Contains(result, "AllowJavaScriptFromAppleEvents = true;") {
		t.Error("Should convert 1 to true")
	}
	if !strings.Contains(result, "ShowStandaloneTabBar = false;") {
		t.Error("Should convert 0 to false")
	}

	// Test array of dictionaries
	if !strings.Contains(result, "FrequentlyVisitedSitesCache = [") {
		t.Error("Should handle array of dictionaries")
	}
	if !strings.Contains(result, "Score = 33.5210800170898;") {
		t.Error("Should handle nested dictionary values")
	}
	if !strings.Contains(result, "Title = \"LinkedIn\";") {
		t.Error("Should handle simple identifiers as strings")
	}

	// Test binary data skipping
	if strings.Contains(result, "customizationSyncServerToken") {
		t.Error("Should skip binary data completely")
	}
}

func TestParseArrayElements(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []Value
	}{
		{
			"Empty string",
			"",
			[]Value{},
		},
		{
			"Single element",
			"hello",
			[]Value{StringValue{Value: "hello"}},
		},
		{
			"Multiple elements",
			"hello, world, test",
			[]Value{
				StringValue{Value: "hello"},
				StringValue{Value: "world"},
				StringValue{Value: "test"},
			},
		},
		{
			"Elements with quotes",
			"\"hello world\", test, \"quoted string\"",
			[]Value{
				StringValue{Value: "hello world"},
				StringValue{Value: "test"},
				StringValue{Value: "quoted string"},
			},
		},
		{
			"Nested structures",
			"{key = value;}, (inner, array), simple",
			[]Value{
				DictValue{
					Values: map[string]Value{"key": StringValue{Value: "value"}},
					Order:  []string{"key"},
				},
				ArrayValue{Values: []Value{
					StringValue{Value: "inner"},
					StringValue{Value: "array"},
				}},
				StringValue{Value: "simple"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseArrayElements(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("parseArrayElements(%q) returned %d elements, want %d", tt.input, len(result), len(tt.expected))
				return
			}
			for i, v := range result {
				if !compareValues(v, tt.expected[i]) {
					t.Errorf("parseArrayElements(%q)[%d] = %v, want %v", tt.input, i, v, tt.expected[i])
				}
			}
		})
	}
}

func TestSkipValue_ToNix(t *testing.T) {
	sv := SkipValue{}
	result := sv.ToNix(0)
	if result != "" {
		t.Errorf("SkipValue.ToNix() = %q, want %q", result, "")
	}
}

// Helper function to compare DictValue maps (since maps can't be compared directly with ==)
func compareDictValues(m1, m2 map[string]Value) bool {
	if len(m1) != len(m2) {
		return false
	}
	for k, v1 := range m1 {
		v2, ok := m2[k]
		if !ok {
			return false
		}
		// Recursively compare Value types
		if !compareValues(v1, v2) {
			return false
		}
	}
	return true
}

// Helper function for deep comparison of Value types
func compareValues(v1, v2 Value) bool {
	switch val1 := v1.(type) {
	case StringValue:
		val2, ok := v2.(StringValue)
		return ok && val1.Value == val2.Value
	case ArrayValue:
		val2, ok := v2.(ArrayValue)
		if !ok || len(val1.Values) != len(val2.Values) {
			return false
		}
		for i := range val1.Values {
			if !compareValues(val1.Values[i], val2.Values[i]) {
				return false
			}
		}
		return true
	case DictValue:
		val2, ok := v2.(DictValue)
		return ok && compareDictValues(val1.Values, val2.Values)
	case SkipValue:
		_, ok := v2.(SkipValue)
		return ok
	default:
		return false
	}
}

func TestComplexNestedStructures(t *testing.T) {
	input := `{
    Level1 = {
        Level2 = {
            Level3 = "deep value";
            Level3Array = (item1, item2, item3);
        };
        SimpleValue = 42;
    };
    TopLevelArray = (
        {
            ArrayDictKey = "array dict value";
            ArrayDictNum = 1;
        },
        "simple array item"
    );
}`

	result, err := convertDefaults(strings.NewReader(input))
	if err != nil {
		t.Fatalf("convertDefaults() error = %v", err)
	}

	// Check nested structure preservation
	if !strings.Contains(result, "Level1 = {") {
		t.Error("Should preserve nested dictionary structure")
	}
	if !strings.Contains(result, "Level3 = \"deep value\"") {
		t.Error("Should handle deeply nested values")
	}
	if !strings.Contains(result, "Level3Array = [") {
		t.Error("Should handle arrays in nested structures")
	}
	if !strings.Contains(result, "TopLevelArray = [") {
		t.Error("Should handle top-level arrays")
	}
	if !strings.Contains(result, "ArrayDictKey = \"array dict value\"") {
		t.Error("Should handle dictionaries within arrays")
	}
}

// Integration tests using actual txt example files
func TestIntegration_SimpleTestFile(t *testing.T) {
	// This test uses the content from simple_test.txt
	input := `{
    AllowJavaScriptFromAppleEvents = 1;
    AutoFillCreditCardData = 1;
    AutoOpenSafeDownloads = 0;
    ShowStandaloneTabBar = 0;
    HomePage = "https://www.apple.com/startpage/";
    ExtensionsEnabled = 1;
}`

	result, err := convertDefaults(strings.NewReader(input))
	if err != nil {
		t.Fatalf("convertDefaults() error = %v", err)
	}

	expected := `{
  AllowJavaScriptFromAppleEvents = true;
  AutoFillCreditCardData = true;
  AutoOpenSafeDownloads = false;
  ShowStandaloneTabBar = false;
  HomePage = "https://www.apple.com/startpage/";
  ExtensionsEnabled = true;
}`

	if result != expected {
		t.Errorf("Integration test failed.\nGot:\n%s\n\nWant:\n%s", result, expected)
	}
}

func TestIntegration_BinaryDataFile(t *testing.T) {
	// This test uses the content from test_binary.txt
	input := `{
    TestSetting = 1;
    HomePage = "https://example.com";
    BinaryData = {length = 256, bytes = 0x89504e47 0d0a1a0a 00000000 49484452};
    AnotherSetting = "value";
    MoreBinaryData = {length = 128, bytes = 0x12345678 abcdef90 deadbeef cafebabe};
    LastSetting = 0;
}`

	result, err := convertDefaults(strings.NewReader(input))
	if err != nil {
		t.Fatalf("convertDefaults() error = %v", err)
	}

	// Verify binary data is skipped completely
	if strings.Contains(result, "BinaryData") || strings.Contains(result, "MoreBinaryData") {
		t.Error("Binary data should be completely skipped from output")
	}

	// Verify other settings are preserved
	expectedContains := []string{
		"TestSetting = true;",
		"HomePage = \"https://example.com\";",
		"AnotherSetting = \"value\";",
		"LastSetting = false;",
	}

	for _, expected := range expectedContains {
		if !strings.Contains(result, expected) {
			t.Errorf("Expected result to contain: %s\nGot: %s", expected, result)
		}
	}
}

func TestIntegration_SafariComplexFile(t *testing.T) {
	// This test uses a subset of the complex Safari configuration from test_safari.txt
	input := `{
    AllowJavaScriptFromAppleEvents = 1;
    AutoFillCreditCardData = 1;
    AutoplayPolicyWhitelistConfigurationUpdateDate = "2025-06-07 12:01:44 +0000";
    ClearBrowsingDataLastIntervalUsed = "today and yesterday";
    CloseTabsAutomatically = 1;
    ExtensionsEnabled = 1;
    "ExtensionsToolbarConfiguration BrowserStandaloneTabBarToolbarIdentifier-v2" = {
        OrderedToolbarItemIdentifiers = (
            CombinedSidebarTabGroupToolbarIdentifier,
            SidebarSeparatorToolbarItemIdentifier,
            BackForwardToolbarIdentifier,
            "com.adguard.safari.AdGuard.Extension (TC3Q7MAJXF) Button"
        );
        UserRemovedToolbarItemIdentifiers = (
        );
    };
    FrequentlyVisitedSitesCache = (
        {
            Score = "33.52108001708984";
            Title = "(282) YouTube";
            URL = "https://www.youtube.com/";
        },
        {
            Score = "13.06611442565918";
            Title = LinkedIn;
            URL = "https://www.linkedin.com/";
        }
    );
    HomePage = "https://www.apple.com/startpage/";
    LastKnownStartPageAppearance = NSAppearanceNameVibrantDark;
    customizationSyncServerToken = {length = 293, bytes = 0x62706c69 73743030 d4010203 04050607};
    ShowStandaloneTabBar = 0;
    "WebKitPreferences.allowsPictureInPictureMediaPlayback" = 1;
}`

	result, err := convertDefaults(strings.NewReader(input))
	if err != nil {
		t.Fatalf("convertDefaults() error = %v", err)
	}

	// Test boolean conversions
	expectedBooleans := map[string]string{
		"AllowJavaScriptFromAppleEvents":                        "true",
		"AutoFillCreditCardData":                                "true",
		"CloseTabsAutomatically":                                "true",
		"ExtensionsEnabled":                                     "true",
		"ShowStandaloneTabBar":                                  "false",
		"WebKitPreferences.allowsPictureInPictureMediaPlayback": "true",
	}

	for key, expectedValue := range expectedBooleans {
		expectedLine := fmt.Sprintf("%s = %s;", key, expectedValue)
		if key == "WebKitPreferences.allowsPictureInPictureMediaPlayback" {
			expectedLine = fmt.Sprintf("\"%s\" = %s;", key, expectedValue)
		}
		if !strings.Contains(result, expectedLine) {
			t.Errorf("Expected result to contain: %s", expectedLine)
		}
	}

	// Test string handling
	if !strings.Contains(result, "AutoplayPolicyWhitelistConfigurationUpdateDate = \"2025-06-07 12:01:44 +0000\";") {
		t.Error("Should handle date strings correctly")
	}
	if !strings.Contains(result, "ClearBrowsingDataLastIntervalUsed = \"today and yesterday\";") {
		t.Error("Should handle strings with spaces correctly")
	}
	if !strings.Contains(result, "HomePage = \"https://www.apple.com/startpage/\";") {
		t.Error("Should handle URL strings correctly")
	}
	if !strings.Contains(result, "LastKnownStartPageAppearance = \"NSAppearanceNameVibrantDark\";") {
		t.Error("Should handle identifier strings correctly")
	}

	// Test complex key handling
	if !strings.Contains(result, "\"ExtensionsToolbarConfiguration BrowserStandaloneTabBarToolbarIdentifier-v2\" = {") {
		t.Error("Should handle complex quoted keys correctly")
	}

	// Test nested structure handling
	if !strings.Contains(result, "OrderedToolbarItemIdentifiers = [") {
		t.Error("Should convert nested arrays correctly")
	}
	if !strings.Contains(result, "UserRemovedToolbarItemIdentifiers = []") {
		t.Error("Should handle empty arrays correctly")
	}

	// Test array of dictionaries
	if !strings.Contains(result, "FrequentlyVisitedSitesCache = [") {
		t.Error("Should handle array of dictionaries")
	}
	if !strings.Contains(result, "Score = 33.5210800170898;") {
		t.Error("Should handle nested dictionary values")
	}
	if !strings.Contains(result, "Title = \"(282) YouTube\";") {
		t.Error("Should handle strings with special characters")
	}
	if !strings.Contains(result, "Title = \"LinkedIn\";") {
		t.Error("Should handle simple identifiers as strings")
	}

	// Test binary data skipping
	if strings.Contains(result, "customizationSyncServerToken") {
		t.Error("Should skip binary data completely")
	}
}

// TestExtractBundleIDs tests the extraction of all top-level keys from parsed values,
// including bundle IDs, NSGlobalDomain, and custom preference domains
func TestExtractBundleIDs(t *testing.T) {
	tests := []struct {
		name     string
		input    Value
		expected []string
	}{
		{
			"Bundle IDs and global domains",
			DictValue{
				Values: map[string]Value{
					"com.apple.Safari":        StringValue{Value: "test"},
					"com.google.Chrome":       StringValue{Value: "test"},
					"NSGlobalDomain":          StringValue{Value: "test"},
					"Custom User Preferences": StringValue{Value: "test"},
					"loginwindow":             StringValue{Value: "test"},
					"Apple Global Domain":     StringValue{Value: "test"},
				},
				Order: []string{"com.apple.Safari", "com.google.Chrome", "NSGlobalDomain", "Custom User Preferences", "loginwindow", "Apple Global Domain"},
			},
			[]string{"com.apple.Safari", "com.google.Chrome", "NSGlobalDomain", "Custom User Preferences", "loginwindow", "Apple Global Domain"},
		},
		{
			"Empty dictionary",
			DictValue{
				Values: map[string]Value{},
				Order:  []string{},
			},
			[]string{},
		},
		{
			"Skip binary data",
			DictValue{
				Values: map[string]Value{
					"com.apple.Safari": StringValue{Value: "test"},
					"binaryData":       SkipValue{},
					"NSGlobalDomain":   StringValue{Value: "test"},
				},
				Order: []string{"com.apple.Safari", "binaryData", "NSGlobalDomain"},
			},
			[]string{"com.apple.Safari", "NSGlobalDomain"},
		},
		{
			"Non-dictionary value",
			StringValue{Value: "not a dict"},
			[]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractBundleIDs(tt.input)

			if len(result) != len(tt.expected) {
				t.Errorf("extractBundleIDs() returned %d keys, want %d", len(result), len(tt.expected))
			}

			for _, expectedKey := range tt.expected {
				if _, exists := result[expectedKey]; !exists {
					t.Errorf("extractBundleIDs() missing expected key: %s", expectedKey)
				}
			}
		})
	}
}

// TestSanitizeFilename tests the conversion of domain keys to safe filenames
// by replacing dots, spaces, and other problematic characters
func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Bundle ID with dots", "com.apple.Safari", "com-apple-Safari"},
		{"Quoted bundle ID", "\"com.google.Chrome\"", "com-google-Chrome"},
		{"NSGlobalDomain", "NSGlobalDomain", "NSGlobalDomain"},
		{"Space in name", "Custom User Preferences", "Custom_User_Preferences"},
		{"Mixed characters", "Apple Global Domain", "Apple_Global_Domain"},
		{"Forward slash", "path/to/something", "path_to_something"},
		{"Complex name", "\"Extension Config v2\"", "Extension_Config_v2"},
		{"loginwindow", "loginwindow", "loginwindow"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeFilename(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeFilename(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestConvertDefaultsWithValue tests the new function that returns both
// the Nix output string and the parsed Value structure for split functionality
func TestConvertDefaultsWithValue(t *testing.T) {
	input := `{
    "com.apple.Safari" = {
        HomePage = "https://example.com";
        ExtensionsEnabled = 1;
    };
    NSGlobalDomain = {
        AppleInterfaceStyle = Dark;
    };
}`

	nixOutput, value, err := convertDefaultsWithValue(input)
	if err != nil {
		t.Fatalf("convertDefaultsWithValue() error = %v", err)
	}

	// Test Nix output
	if !strings.Contains(nixOutput, "com.apple.Safari") {
		t.Error("Nix output should contain Safari bundle ID")
	}
	if !strings.Contains(nixOutput, "NSGlobalDomain") {
		t.Error("Nix output should contain NSGlobalDomain")
	}

	// Test returned value structure
	if dict, ok := value.(DictValue); ok {
		if len(dict.Values) != 2 {
			t.Errorf("Expected 2 top-level keys, got %d", len(dict.Values))
		}
		if _, exists := dict.Values["\"com.apple.Safari\""]; !exists {
			if _, exists := dict.Values["com.apple.Safari"]; !exists {
				t.Error("Should contain Safari bundle ID in parsed value")
			}
		}
		if _, exists := dict.Values["NSGlobalDomain"]; !exists {
			t.Error("Should contain NSGlobalDomain in parsed value")
		}
	} else {
		t.Error("Expected DictValue from convertDefaultsWithValue")
	}
}

// TestSplitFunctionality_Integration tests the complete split workflow
// including parsing, key extraction, and individual file content generation
func TestDateOmission(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		noDates  bool
		expected string
	}{
		{
			name:     "Date string omitted when noDates is true",
			input:    `"2025-06-07 12:01:44 +0000"`,
			noDates:  true,
			expected: "",
		},
		{
			name:     "Date string preserved when noDates is false",
			input:    `"2025-06-07 12:01:44 +0000"`,
			noDates:  false,
			expected: `"2025-06-07 12:01:44 +0000"`,
		},
		{
			name: "Dictionary with date values omitted",
			input: `{
				UpdateDate = "2025-06-07 12:01:44 +0000";
				Version = "1.2.3";
				LastModified = "2024-12-15 08:30:00 +0000";
			}`,
			noDates: true,
			expected: `{
  Version = "1.2.3";
}`,
		},
		{
			name: "Array with mixed values",
			input: `(
				"2025-06-07 12:01:44 +0000",
				"normal string",
				"2024-01-01T10:00:00Z",
				42
			)`,
			noDates: true,
			expected: `[
  "normal string"
  42
]`,
		},
		{
			name:     "ISO 8601 date format",
			input:    `"2025-06-07T12:01:44Z"`,
			noDates:  true,
			expected: "",
		},
		{
			name:     "Date only format",
			input:    `"2025-06-07"`,
			noDates:  true,
			expected: "",
		},
		{
			name:     "Non-date string preserved",
			input:    `"This is not a date: 2025-06-07"`,
			noDates:  true,
			expected: `"This is not a date: 2025-06-07"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := convertDefaultsWithValueAndConfig(tt.input, ParseConfig{NoDates: tt.noDates})
			if err != nil {
				t.Fatalf("Error converting: %v", err)
			}

			result = strings.TrimSpace(result)
			expected := strings.TrimSpace(tt.expected)

			if result != expected {
				t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
			}
		})
	}
}

func TestIsDateString(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"2025-06-07 12:01:44 +0000", true},
		{"2025-06-07T12:01:44Z", true},
		{"2025-06-07", true},
		{"2025-06-07T12:01:44+08:00", true},
		{"not a date", false},
		{"2025 is a year", false},
		{"12:34:56", false},
		{"", false},
		{"2025/06/07", false}, // Wrong separator
		{"2025-99-99", false}, // Invalid month/day
		{"2025-13-01", false}, // Invalid month
		{"2025-01-32", false}, // Invalid day
		{"1800-01-01", false}, // Year too old
		{"2200-01-01", false}, // Year too far in future
		{"2025-01-01 25:00:00 +0000", false}, // Invalid time
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := isDateString(tt.input)
			if result != tt.expected {
				t.Errorf("isDateString(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsBinaryDataValue(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "Valid binary data",
			input:    `{length = 256; bytes = 0x89504e47;}`,
			expected: true,
		},
		{
			name:     "Valid binary data with whitespace",
			input:    `{ length = 32; bytes = 0xdeadbeef; }`,
			expected: true,
		},
		{
			name:     "Valid binary data with comma separator",
			input:    `{length = 256, bytes = 0x89504e47 0d0a1a0a}`,
			expected: true,
		},
		{
			name:     "Not binary data - regular dict",
			input:    `{name = "test"; value = 42;}`,
			expected: false,
		},
		{
			name:     "Dict with length but no bytes",
			input:    `{length = 256; name = "test";}`,
			expected: false,
		},
		{
			name:     "Dict with bytes but wrong format",
			input:    `{length = 256; bytes = "not hex";}`,
			expected: false,
		},
		{
			name:     "Dict with extra keys",
			input:    `{length = 256; bytes = 0x1234; extra = "data";}`,
			expected: false,
		},
		{
			name:     "Empty dict",
			input:    `{}`,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isBinaryDataValue(tt.input)
			if result != tt.expected {
				t.Errorf("isBinaryDataValue(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSplitFunctionality_Integration(t *testing.T) {
	// Test the main logic that would be used in split mode
	input := `{
    "com.apple.Safari" = {
        HomePage = "https://example.com";
        ExtensionsEnabled = 1;
    };
    NSGlobalDomain = {
        AppleInterfaceStyle = Dark;
        AppleLanguages = ("en-US", "en");
    };
    "Custom User Preferences" = {
        MyCustomSetting = enabled;
    };
    loginwindow = {
        LoginwindowText = Welcome;
    };
}`

	_, value, err := convertDefaultsWithValue(input)
	if err != nil {
		t.Fatalf("convertDefaultsWithValue() error = %v", err)
	}

	bundleMap := extractBundleIDs(value)

	expectedKeys := []string{"\"com.apple.Safari\"", "NSGlobalDomain", "\"Custom User Preferences\"", "loginwindow"}
	alternateKeys := []string{"com.apple.Safari", "NSGlobalDomain", "Custom User Preferences", "loginwindow"}

	if len(bundleMap) != len(expectedKeys) {
		t.Errorf("Expected %d keys, got %d", len(expectedKeys), len(bundleMap))
	}

	for i, key := range expectedKeys {
		if _, exists := bundleMap[key]; !exists {
			if _, exists := bundleMap[alternateKeys[i]]; !exists {
				t.Errorf("Expected key %s (or %s) not found in bundle map", key, alternateKeys[i])
			}
		}
	}

	// Test individual key content - check both quoted and unquoted versions
	var safariValue Value
	var safariExists bool
	if safariValue, safariExists = bundleMap["\"com.apple.Safari\""]; !safariExists {
		safariValue, safariExists = bundleMap["com.apple.Safari"]
	}

	if safariExists {
		safariNix := safariValue.ToNix(0)
		if !strings.Contains(safariNix, "HomePage = \"https://example.com\"") {
			t.Error("Safari config should contain HomePage setting")
		}
		if !strings.Contains(safariNix, "ExtensionsEnabled = true") {
			t.Error("Safari config should contain ExtensionsEnabled setting")
		}
	}

	if globalValue, exists := bundleMap["NSGlobalDomain"]; exists {
		globalNix := globalValue.ToNix(0)
		if !strings.Contains(globalNix, "AppleInterfaceStyle = \"Dark\"") {
			t.Error("NSGlobalDomain should contain AppleInterfaceStyle setting")
		}
		if !strings.Contains(globalNix, "AppleLanguages = [") {
			t.Error("NSGlobalDomain should contain AppleLanguages array")
		}
	}
}

// TestParseValue_MalformedInputs tests error handling for malformed inputs
func TestParseValue_MalformedInputs(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectPanic bool
	}{
		{"Unmatched opening brace", "{key = value", false},
		{"Unmatched closing brace", "key = value}", false},
		{"Unmatched opening paren", "(item1, item2", false},
		{"Unmatched closing paren", "item1, item2)", false},
		{"Malformed dict - no equals", "{key value;}", false},
		{"Malformed dict - no semicolon", "{key = value}", false},
		{"Unterminated quote", "\"unterminated string", false},
		{"Double quote in middle", "test\"quote", false},
		{"Empty input", "", false},
		{"Just whitespace", "   \n  \t  ", false},
		{"Invalid escape sequence", "\"test\\q\"", false},
		{"Nested malformed dict", "{outer = {inner = }; }", false},
		{"Deeply nested malformed", "{a = {b = {c = }; }; }", false},
		{"Mixed quotes", "\"'mixed'\"", false},
		{"Unicode control chars", "test\x00\x01\x02", false},
		{"Very long string", strings.Repeat("a", 100000), false},
		{"Circular-like structure", "{a = b; b = a;}", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expectPanic {
				defer func() {
					if r := recover(); r == nil {
						t.Errorf("Expected panic for input %q", tt.input)
					}
				}()
			}
			
			// Test that malformed input doesn't crash - should return something
			result := parseValue(tt.input)
			if result == nil {
				t.Errorf("parseValue(%q) returned nil", tt.input)
			}
			
			// Test that conversion doesn't crash either
			nixStr := result.ToNix(0)
			if nixStr == "" && tt.input != "" && strings.TrimSpace(tt.input) != "" {
				// Allow empty results for truly empty inputs
				t.Logf("parseValue(%q) produced empty Nix output: %q", tt.input, nixStr)
			}
		})
	}
}

// TestConvertDefaults_MalformedInputs tests error handling at the conversion level
func TestConvertDefaults_MalformedInputs(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"Incomplete dict", "{\n  key = value\n"},       // Missing closing brace
		{"Invalid syntax", "key = = value;"},              // Double equals
		{"Broken array", "(item1, item2,)"},               // Trailing comma
		{"Mixed delimiters", "{key = value,}"},            // Comma instead of semicolon
		{"Nested incomplete", "{outer = {inner = ; };}"}, // Missing value
		{"Invalid UTF-8", "{\x80\x81\x82 = value;}"},     // Invalid UTF-8
		{"CR/LF issues", "{\r\nkey\r = \rvalue\r;\r\n}"}, // Various line endings
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := convertDefaults(strings.NewReader(tt.input))
			// We expect the function to handle errors gracefully
			// It should either succeed with best-effort parsing or return a meaningful error
			if err != nil {
				t.Logf("convertDefaults(%q) returned expected error: %v", tt.name, err)
			} else {
				// If no error, ensure we get some valid Nix output
				if result == "" {
					t.Errorf("convertDefaults(%q) returned empty result without error", tt.name)
				}
				t.Logf("convertDefaults(%q) succeeded with result: %q", tt.name, result)
			}
		})
	}
}

// TestParseArray_EdgeCases tests array parsing with problematic inputs
func TestParseArray_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int // Expected number of elements
	}{
		{"Empty array", "()", 0},
		{"Array with only whitespace", "(   )", 0},
		{"Single empty element", "(\"\",)", 1},
		{"Trailing comma", "(a, b, c,)", 3},
		{"Multiple commas", "(a,, b)", 2}, // Should handle double comma gracefully
		{"Unquoted complex strings", "(item-with-dash, item.with.dot)", 2},
		{"Mixed empty and full", "(\"\", value, \"\")", 3},
		{"Nested empty arrays", "((), (a, b), ())", 3},
		{"Deeply nested", "(((nested)))", 1},
		{"Array with semicolons", "(a; b; c)", 1}, // Semicolons shouldn't split array elements
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseArray(tt.input)
			if len(result.Values) != tt.expected {
				t.Errorf("parseArray(%q) returned %d elements, want %d", 
					tt.input, len(result.Values), tt.expected)
			}
			
			// Ensure ToNix doesn't crash
			nixOutput := result.ToNix(0)
			if nixOutput == "" && tt.expected > 0 {
				t.Errorf("parseArray(%q).ToNix() returned empty string for non-empty array", tt.input)
			}
		})
	}
}

// TestParseDict_EdgeCases tests dictionary parsing with problematic inputs
func TestParseDict_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectEmpty bool
	}{
		{"Empty dict", "{}", true},
		{"Dict with only whitespace", "{   }", true},
		{"Single key no value", "{key = ;}", false},
		{"Key with no equals", "{key value;}", false},
		{"Multiple equals", "{key = = value;}", false},
		{"Missing semicolon", "{key = value}", false},
		{"Trailing semicolon", "{key = value;;}", false},
		{"Empty key", "{ = value;}", false},
		{"Quoted empty key", "{\"\" = value;}", false},
		{"Key with special chars", "{\"key with spaces and = signs\" = value;}", false},
		{"Unicode in key", "{\"key🚀test\" = value;}", false},
		{"Very long key", fmt.Sprintf("{\"%s\" = value;}", strings.Repeat("k", 1000)), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseDict(tt.input)
			
			if tt.expectEmpty && len(result.Values) != 0 {
				t.Errorf("parseDict(%q) expected empty, got %d values", tt.input, len(result.Values))
			}
			
			// Ensure ToNix doesn't crash
			nixOutput := result.ToNix(0)
			if nixOutput == "" && !tt.expectEmpty && len(result.Values) > 0 {
				t.Errorf("parseDict(%q).ToNix() returned empty string unexpectedly", tt.input)
			}
		})
	}
}

// TestStringValue_SpecialCases tests StringValue with edge cases
func TestStringValue_SpecialCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Empty string", "", "\"\""},
		{"Only whitespace", "   ", "\"   \""},
		{"Tab characters", "\t\t", "\"\t\t\""},
		{"Newline characters", "\n", "\"\n\""},
		{"Unicode emoji", "🚀", "\"🚀\""},
		{"Unicode combining chars", "é", "\"é\""},
		{"Very long string", strings.Repeat("x", 10000), fmt.Sprintf("\"%s\"", strings.Repeat("x", 10000))},
		{"All digits but not number", "00123", "123"}, // Leading zeros are lost in int parsing
		{"Floating point edge", "3.14159265358979323846", "3.14159265358979"}, // Float precision limit
		{"Scientific notation", "1.23e10", "12300000000"},
		{"Negative number", "-42", "-42"},
		{"Zero", "0", "false"}, // Special boolean case
		{"One", "1", "true"},   // Special boolean case
		{"Two", "2", "2"},      // Not a boolean
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sv := StringValue{Value: tt.input}
			result := sv.ToNix(0)
			if result != tt.expected {
				t.Errorf("StringValue{%q}.ToNix() = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestCLI_FlagValidation tests command-line flag combinations by calling the binary
func TestCLI_FlagValidation(t *testing.T) {
	// Skip on non-Darwin platforms since the tool requires macOS
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping CLI tests on non-Darwin platform")
	}

	// Build the binary for testing
	tempDir := t.TempDir()
	binaryPath := tempDir + "/defaults2nix-test"
	
	buildCmd := exec.Command("go", "build", "-o", binaryPath)
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build test binary: %v", err)
	}

	tests := []struct {
		name           string
		args           []string
		expectExitCode int
		expectStderr   string
	}{
		{
			name:           "No arguments shows usage",
			args:           []string{},
			expectExitCode: 1,
			expectStderr:   "Usage:",
		},
		{
			name:           "Help flag works",
			args:           []string{"-h"},
			expectExitCode: 0, // -h flag exits with 0
			expectStderr:   "Usage:",
		},
		{
			name:           "Invalid flag combination: -all with domain",
			args:           []string{"-all", "com.apple.Safari"},
			expectExitCode: 1,
			expectStderr:   "Cannot use -all or -split with a domain argument",
		},
		{
			name:           "Invalid flag combination: -split with domain",
			args:           []string{"-split", "com.apple.Safari"},
			expectExitCode: 1,
			expectStderr:   "Cannot use -all or -split with a domain argument",
		},
		{
			name:           "Invalid flag combination: -all and -split together",
			args:           []string{"-all", "-split"},
			expectExitCode: 1,
			expectStderr:   "Cannot use -all and -split at the same time",
		},
		{
			name:           "Missing -out with -split",
			args:           []string{"-split"},
			expectExitCode: 1,
			expectStderr:   "-out is mandatory when -split is used",
		},
		{
			name:           "Invalid flag",
			args:           []string{"-invalid-flag"},
			expectExitCode: 2, // Flag package exits with 2 for invalid flags
			expectStderr:   "flag provided but not defined",
		},
		{
			name:           "Multiple domains uses first one",
			args:           []string{"com.apple.Safari", "com.apple.dock"},
			expectExitCode: 0, // Actually succeeds, just uses first domain
			expectStderr:   "", // No error - this is valid behavior
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(binaryPath, tt.args...)
			output, err := cmd.CombinedOutput()
			
			exitCode := 0
			if err != nil {
				if exitError, ok := err.(*exec.ExitError); ok {
					exitCode = exitError.ExitCode()
				}
			}

			if exitCode != tt.expectExitCode {
				t.Errorf("Expected exit code %d, got %d", tt.expectExitCode, exitCode)
				t.Logf("Command output: %s", string(output))
			}

			if tt.expectStderr != "" && !strings.Contains(string(output), tt.expectStderr) {
				t.Errorf("Expected stderr to contain %q, got: %s", tt.expectStderr, string(output))
			}
		})
	}
}

// TestCLI_PlatformCheck tests that the tool properly checks for macOS
func TestCLI_PlatformCheck(t *testing.T) {
	// This test is meaningful only on non-Darwin platforms
	if runtime.GOOS == "darwin" {
		t.Skip("Skipping platform check test on Darwin platform")
	}

	// Build the binary for testing
	tempDir := t.TempDir()
	binaryPath := tempDir + "/defaults2nix-test"
	
	buildCmd := exec.Command("go", "build", "-o", binaryPath)
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build test binary: %v", err)
	}

	cmd := exec.Command(binaryPath, "com.apple.Safari")
	output, err := cmd.CombinedOutput()
	
	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		}
	}

	if exitCode != 1 {
		t.Errorf("Expected exit code 1 for non-macOS platform, got %d", exitCode)
	}

	expectedMessages := []string{
		"designed for macOS only",
		"requires 'defaults' command",
		"Current platform:",
	}

	outputStr := string(output)
	for _, expected := range expectedMessages {
		if !strings.Contains(outputStr, expected) {
			t.Errorf("Expected output to contain %q, got: %s", expected, outputStr)
		}
	}
}

// TestCLI_OutputFileValidation tests output file validation
func TestCLI_OutputFileValidation(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping CLI tests on non-Darwin platform")
	}

	tempDir := t.TempDir()
	binaryPath := tempDir + "/defaults2nix-test"
	
	buildCmd := exec.Command("go", "build", "-o", binaryPath)
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build test binary: %v", err)
	}

	// Create a file to test directory vs file validation
	testFile := tempDir + "/testfile"
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tests := []struct {
		name           string
		args           []string
		expectExitCode int
		expectStderr   string
	}{
		{
			name:           "Split with file instead of directory",
			args:           []string{"-split", "-out", testFile},
			expectExitCode: 1,
			expectStderr:   "must be a directory when -split is used",
		},
		{
			name:           "Non-split with directory instead of file",
			args:           []string{"-all", "-out", tempDir},
			expectExitCode: 1,
			expectStderr:   "must be a file when not using -split",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(binaryPath, tt.args...)
			output, err := cmd.CombinedOutput()
			
			exitCode := 0
			if err != nil {
				if exitError, ok := err.(*exec.ExitError); ok {
					exitCode = exitError.ExitCode()
				}
			}

			if exitCode != tt.expectExitCode {
				t.Errorf("Expected exit code %d, got %d", tt.expectExitCode, exitCode)
				t.Logf("Command output: %s", string(output))
			}

			if tt.expectStderr != "" && !strings.Contains(string(output), tt.expectStderr) {
				t.Errorf("Expected stderr to contain %q, got: %s", tt.expectStderr, string(output))
			}
		})
	}
}

// TestFileOperations_ErrorHandling tests file I/O error scenarios
func TestFileOperations_ErrorHandling(t *testing.T) {
	tempDir := t.TempDir()
	
	tests := []struct {
		name        string
		setupFunc   func() string // Returns file/dir path to test
		expectError bool
	}{
		{
			name: "Write to read-only directory",
			setupFunc: func() string {
				readOnlyDir := tempDir + "/readonly"
				if err := os.Mkdir(readOnlyDir, 0444); err != nil {
					t.Fatalf("Failed to create readonly dir: %v", err)
				}
				return readOnlyDir + "/test.nix"
			},
			expectError: true,
		},
		{
			name: "Write to non-existent directory path",
			setupFunc: func() string {
				return tempDir + "/nonexistent/path/file.nix"
			},
			expectError: true,
		},
		{
			name: "Write to existing file with different permissions",
			setupFunc: func() string {
				restrictedFile := tempDir + "/restricted.nix"
				if err := os.WriteFile(restrictedFile, []byte("test"), 0000); err != nil {
					t.Fatalf("Failed to create restricted file: %v", err)
				}
				return restrictedFile
			},
			expectError: true,
		},
		{
			name: "Write to valid path",
			setupFunc: func() string {
				return tempDir + "/valid.nix"
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath := tt.setupFunc()
			
			// Test writing a simple Nix configuration
			testContent := `{
  TestSetting = true;
  HomePage = "https://example.com";
}`
			
			err := os.WriteFile(filePath, []byte(testContent), 0644)
			
			if tt.expectError && err == nil {
				t.Errorf("Expected error writing to %s, but succeeded", filePath)
			}
			
			if !tt.expectError && err != nil {
				t.Errorf("Expected success writing to %s, but got error: %v", filePath, err)
			}
		})
	}
}

// TestDirectoryOperations_ErrorHandling tests directory creation and validation
func TestDirectoryOperations_ErrorHandling(t *testing.T) {
	tempDir := t.TempDir()
	
	tests := []struct {
		name        string
		setupFunc   func() string // Returns directory path to test
		expectError bool
	}{
		{
			name: "Create directory in read-only parent",
			setupFunc: func() string {
				readOnlyParent := tempDir + "/readonly_parent"
				if err := os.Mkdir(readOnlyParent, 0444); err != nil {
					t.Fatalf("Failed to create readonly parent: %v", err)
				}
				return readOnlyParent + "/newdir"
			},
			expectError: true,
		},
		{
			name: "Create nested directory path",
			setupFunc: func() string {
				return tempDir + "/nested/deep/path"
			},
			expectError: false,
		},
		{
			name: "Create directory where file exists",
			setupFunc: func() string {
				existingFile := tempDir + "/existing_file"
				if err := os.WriteFile(existingFile, []byte("test"), 0644); err != nil {
					t.Fatalf("Failed to create existing file: %v", err)
				}
				return existingFile
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dirPath := tt.setupFunc()
			
			err := os.MkdirAll(dirPath, 0755)
			
			if tt.expectError && err == nil {
				t.Errorf("Expected error creating directory %s, but succeeded", dirPath)
			}
			
			if !tt.expectError && err != nil {
				t.Errorf("Expected success creating directory %s, but got error: %v", dirPath, err)
			}
		})
	}
}

// TestConvertDefaults_IOErrors tests I/O error handling in conversion functions
func TestConvertDefaults_IOErrors(t *testing.T) {
	tests := []struct {
		name      string
		readerFunc func() *strings.Reader
		expectError bool
	}{
		{
			name: "Valid input reader",
			readerFunc: func() *strings.Reader {
				return strings.NewReader(`{TestSetting = 1;}`)
			},
			expectError: false,
		},
		{
			name: "Empty reader",
			readerFunc: func() *strings.Reader {
				return strings.NewReader("")
			},
			expectError: false, // Should handle empty input gracefully
		},
		{
			name: "Large input within scanner limits",
			readerFunc: func() *strings.Reader {
				// Create a large but valid input that won't exceed bufio.Scanner limits
				large := strings.Repeat("TestKey = \""+strings.Repeat("x", 100)+"\"; ", 50)
				return strings.NewReader("{" + large + "}")
			},
			expectError: false,
		},
		{
			name: "Very large input exceeding scanner limits",
			readerFunc: func() *strings.Reader {
				// Create input that exceeds bufio.Scanner token limits (typically 64KB)
				large := strings.Repeat("TestKey = \""+strings.Repeat("x", 10000)+"\"; ", 100)
				return strings.NewReader("{" + large + "}")
			},
			expectError: true, // This should fail with "token too long" error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := tt.readerFunc()
			
			result, err := convertDefaults(reader)
			
			if tt.expectError && err == nil {
				t.Errorf("Expected error from convertDefaults, but succeeded")
			}
			
			if !tt.expectError && err != nil {
				t.Errorf("Expected success from convertDefaults, but got error: %v", err)
			}
			
			if !tt.expectError && result == "" {
				t.Errorf("Expected non-empty result from convertDefaults")
			}
		})
	}
}

// TestSplitMode_FileOperationErrors tests error scenarios in split mode file operations
func TestSplitMode_FileOperationErrors(t *testing.T) {
	tempDir := t.TempDir()
	
	// Test the core logic that split mode uses for file operations
	bundleData := map[string]Value{
		"com.apple.Safari": DictValue{
			Values: map[string]Value{
				"HomePage": StringValue{Value: "https://example.com"},
			},
			Order: []string{"HomePage"},
		},
		"NSGlobalDomain": DictValue{
			Values: map[string]Value{
				"AppleInterfaceStyle": StringValue{Value: "Dark"},
			},
			Order: []string{"AppleInterfaceStyle"},
		},
	}

	tests := []struct {
		name        string
		setupFunc   func() string // Returns output directory path
		expectError bool
	}{
		{
			name: "Valid output directory",
			setupFunc: func() string {
				validDir := tempDir + "/valid_output"
				if err := os.Mkdir(validDir, 0755); err != nil {
					t.Fatalf("Failed to create valid dir: %v", err)
				}
				return validDir
			},
			expectError: false,
		},
		{
			name: "Read-only output directory", 
			setupFunc: func() string {
				readOnlyDir := tempDir + "/readonly_output"
				if err := os.Mkdir(readOnlyDir, 0444); err != nil {
					t.Fatalf("Failed to create readonly dir: %v", err)
				}
				return readOnlyDir
			},
			expectError: true,
		},
		{
			name: "Output directory is actually a file",
			setupFunc: func() string {
				filePath := tempDir + "/not_a_dir"
				if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
					t.Fatalf("Failed to create file: %v", err)
				}
				return filePath
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputDir := tt.setupFunc()
			
			// Simulate split mode file writing
			var writeErrors []error
			for bundleID, bundleValue := range bundleData {
				nixContent := bundleValue.ToNix(0)
				filename := outputDir + "/" + sanitizeFilename(bundleID) + ".nix"
				
				err := os.WriteFile(filename, []byte(nixContent), 0644)
				if err != nil {
					writeErrors = append(writeErrors, err)
				}
			}

			hasErrors := len(writeErrors) > 0
			
			if tt.expectError && !hasErrors {
				t.Errorf("Expected file write errors, but all writes succeeded")
			}
			
			if !tt.expectError && hasErrors {
				t.Errorf("Expected successful file writes, but got errors: %v", writeErrors)
			}
		})
	}
}

// TestCommandExecution_FailureHandling tests handling of defaults command failures
func TestCommandExecution_FailureHandling(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping command execution tests on non-Darwin platform")
	}

	tempDir := t.TempDir()
	binaryPath := tempDir + "/defaults2nix-test"
	
	buildCmd := exec.Command("go", "build", "-o", binaryPath)
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build test binary: %v", err)
	}

	tests := []struct {
		name           string
		args           []string
		expectExitCode int
		expectStderr   string
	}{
		{
			name:           "Invalid domain name",
			args:           []string{"com.nonexistent.invalid.domain.that.does.not.exist"},
			expectExitCode: 1,
			expectStderr:   "Error executing 'defaults read",
		},
		{
			name:           "Domain with special characters",
			args:           []string{"invalid$domain@name"},
			expectExitCode: 1,
			expectStderr:   "Error executing 'defaults read",
		},
		{
			name:           "Empty domain name",
			args:           []string{""},
			expectExitCode: 1,
			expectStderr:   "Error executing 'defaults read",
		},
		{
			name:           "Very long domain name",
			args:           []string{strings.Repeat("a", 1000) + ".domain"},
			expectExitCode: 1,
			expectStderr:   "Error executing 'defaults read",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(binaryPath, tt.args...)
			output, err := cmd.CombinedOutput()
			
			exitCode := 0
			if err != nil {
				if exitError, ok := err.(*exec.ExitError); ok {
					exitCode = exitError.ExitCode()
				}
			}

			if exitCode != tt.expectExitCode {
				t.Errorf("Expected exit code %d, got %d", tt.expectExitCode, exitCode)
				t.Logf("Command output: %s", string(output))
			}

			if tt.expectStderr != "" && !strings.Contains(string(output), tt.expectStderr) {
				t.Errorf("Expected stderr to contain %q, got: %s", tt.expectStderr, string(output))
			}
		})
	}
}

// TestSplitMode_DomainCommandFailures tests split mode behavior when defaults commands fail
func TestSplitMode_DomainCommandFailures(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping split mode tests on non-Darwin platform")
	}

	tempDir := t.TempDir()
	binaryPath := tempDir + "/defaults2nix-test"
	outputDir := tempDir + "/output"
	
	buildCmd := exec.Command("go", "build", "-o", binaryPath)
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build test binary: %v", err)
	}

	if err := os.Mkdir(outputDir, 0755); err != nil {
		t.Fatalf("Failed to create output directory: %v", err)
	}

	// Test split mode - this will likely encounter some domains that fail
	// but should continue processing others
	cmd := exec.Command(binaryPath, "-split", "-out", outputDir)
	output, err := cmd.CombinedOutput()
	
	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		}
	}

	// Split mode should either succeed (if some domains work) or fail gracefully
	outputStr := string(output)
	
	if exitCode == 0 {
		// If successful, should have informative output
		if !strings.Contains(outputStr, "Successfully processed") {
			t.Errorf("Expected success message in output, got: %s", outputStr)
		}
	} else if exitCode == 1 {
		// If failed, should have error information
		if !strings.Contains(outputStr, "Error:") && !strings.Contains(outputStr, "No domains could be processed") {
			t.Errorf("Expected error message in output, got: %s", outputStr)
		}
	} else {
		t.Errorf("Unexpected exit code %d, got output: %s", exitCode, outputStr)
	}
}

// MockReader simulates various I/O error conditions
type MockReader struct {
	data       []byte
	position   int
	errorAfter int // Trigger error after reading this many bytes
}

func (m *MockReader) Read(p []byte) (n int, err error) {
	if m.errorAfter >= 0 && m.position >= m.errorAfter {
		return 0, fmt.Errorf("simulated read error")
	}
	
	remaining := len(m.data) - m.position
	if remaining == 0 {
		return 0, fmt.Errorf("EOF") // io.EOF would be better but this is for testing errors
	}
	
	n = copy(p, m.data[m.position:])
	m.position += n
	return n, nil
}

// TestConvertDefaults_ReadErrors tests handling of reader errors during conversion
func TestConvertDefaults_ReadErrors(t *testing.T) {
	tests := []struct {
		name        string
		readerFunc  func() *MockReader
		expectError bool
	}{
		{
			name: "Error after reading some data",
			readerFunc: func() *MockReader {
				return &MockReader{
					data:       []byte(`{TestSetting = 1; AnotherSetting = 2;`),
					errorAfter: 20, // Error partway through
				}
			},
			expectError: true,
		},
		{
			name: "Error immediately",
			readerFunc: func() *MockReader {
				return &MockReader{
					data:       []byte(`{TestSetting = 1;}`),
					errorAfter: 0, // Error immediately
				}
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := tt.readerFunc()
			
			_, err := convertDefaults(reader)
			
			if tt.expectError && err == nil {
				t.Errorf("Expected error from convertDefaults with failing reader, but succeeded")
			}
			
			if !tt.expectError && err != nil {
				t.Errorf("Expected success from convertDefaults, but got error: %v", err)
			}
		})
	}
}

// TestSystemIntegration_RealWorldScenarios tests with realistic macOS defaults data
func TestSystemIntegration_RealWorldScenarios(t *testing.T) {
	// This test uses complex, real-world-like data structures
	complexInput := `{
    "com.apple.Safari" = {
        AllowJavaScriptFromAppleEvents = 1;
        AutoFillCreditCardData = 1;
        AutoOpenSafeDownloads = 0;
        AutoplayPolicyWhitelistConfigurationUpdateDate = "2025-06-07 12:01:44 +0000";
        BookmarksBarShowsAddressBarSuggestion = 1;
        ClearBrowsingDataLastIntervalUsed = "today and yesterday";
        DownloadsClearancePolicy = 2;
        ExtensionsEnabled = 1;
        "ExtensionsToolbarConfiguration BrowserStandaloneTabBarToolbarIdentifier-v2" = {
            OrderedToolbarItemIdentifiers = (
                CombinedSidebarTabGroupToolbarIdentifier,
                SidebarSeparatorToolbarItemIdentifier,
                BackForwardToolbarIdentifier,
                NSToolbarFlexibleSpaceItemIdentifier,
                "com.adguard.safari.AdGuard.Extension (TC3Q7MAJXF) Button"
            );
            UserRemovedToolbarItemIdentifiers = (
            );
        };
        FrequentlyVisitedSitesCache = (
            {
                LastVisitTime = "2025-06-07T15:30:42Z";
                Score = "33.52108001708984";
                Title = "(282) YouTube";
                URL = "https://www.youtube.com/";
            },
            {
                LastVisitTime = "2025-06-06T10:15:30Z";
                Score = "13.06611442565918";
                Title = LinkedIn;
                URL = "https://www.linkedin.com/";
            }
        );
        GenericPasswordManager = {
            autofillAttempted = 1;
            passwords = {
                length = 4096;
                bytes = 0x62706c69 73743030 d4010203 04050607 08091011 1213143c 61726368 69766572;
            };
            shouldSavePasswords = 1;
        };
        HomePage = "https://www.apple.com/startpage/";
        LastKnownStartPageAppearance = NSAppearanceNameVibrantDark;
        ShowStandaloneTabBar = 0;
        "WebKitPreferences.allowsPictureInPictureMediaPlayback" = 1;
        "WebKitPreferences.javaScriptEnabled" = 1;
        customizationSyncServerToken = {
            length = 293;
            bytes = 0x62706c69 73743030 d4010203 04050607 08091011 1213143c 61726368 69766572;
        };
    };
    NSGlobalDomain = {
        AppleAccentColor = 1;
        AppleActionOnDoubleClick = Maximize;
        AppleAquaColorVariant = 6;
        AppleHighlightColor = "0.968627 0.831373 1.000000 Purple";
        AppleICUForce24HourTime = 0;
        AppleInterfaceStyle = Dark;
        AppleInterfaceStyleSwitchesAutomatically = 0;
        AppleKeyboardUIMode = 3;
        AppleLanguages = (
            "en-US",
            en
        );
        AppleLocale = "en_US";
        AppleMiniaturizeOnDoubleClick = 1;
        AppleScrollerPagingBehavior = 1;
        AppleShowAllExtensions = 1;
        AppleShowScrollBars = Automatic;
        InitialKeyRepeat = 25;
        KeyRepeat = 2;
        NSDocumentSaveNewDocumentsToCloud = 0;
        NSNavPanelExpandedStateForSaveMode = 1;
        NSQuitAlwaysKeepsWindows = 0;
        NSScrollAnimationEnabled = 1;
        NSTableViewDefaultSizeMode = 2;
        NSToolbarTitleViewRolloverDelay = "0.5";
        NSUserKeyEquivalents = {
            "Target Display Mode" = "@~F1";
        };
        PMPrintingExpandedStateForPrint2 = 1;
        WebKitDeveloperExtras = 1;
    };
}`

	tests := []struct {
		name     string
		config   ParseConfig
		validate func(string, Value) error
	}{
		{
			name:   "Full conversion with all features",
			config: ParseConfig{NoDates: false},
			validate: func(nixOutput string, parsedValue Value) error {
				// Check basic structure
				if !strings.Contains(nixOutput, "com.apple.Safari") {
					return fmt.Errorf("missing Safari configuration")
				}
				if !strings.Contains(nixOutput, "NSGlobalDomain") {
					return fmt.Errorf("missing NSGlobalDomain configuration")
				}
				
				// Check boolean conversions
				if !strings.Contains(nixOutput, "AllowJavaScriptFromAppleEvents = true;") {
					return fmt.Errorf("boolean conversion failed")
				}
				if !strings.Contains(nixOutput, "AutoOpenSafeDownloads = false;") {
					return fmt.Errorf("boolean conversion failed")
				}
				
				// Check complex key quoting
				if !strings.Contains(nixOutput, "\"ExtensionsToolbarConfiguration BrowserStandaloneTabBarToolbarIdentifier-v2\"") {
					return fmt.Errorf("complex key quoting failed")
				}
				if !strings.Contains(nixOutput, "\"WebKitPreferences.allowsPictureInPictureMediaPlayback\"") {
					return fmt.Errorf("dotted key quoting failed")
				}
				
				// Check nested arrays and dicts
				if !strings.Contains(nixOutput, "FrequentlyVisitedSitesCache = [") {
					return fmt.Errorf("nested array conversion failed")
				}
				if !strings.Contains(nixOutput, "OrderedToolbarItemIdentifiers = [") {
					return fmt.Errorf("nested array in dict failed")
				}
				
				// Check binary data removal
				if strings.Contains(nixOutput, "customizationSyncServerToken") || strings.Contains(nixOutput, "passwords") {
					return fmt.Errorf("binary data should be filtered out")
				}
				
				// Check date handling
				if !strings.Contains(nixOutput, "AutoplayPolicyWhitelistConfigurationUpdateDate") {
					return fmt.Errorf("date string should be preserved when NoDates is false")
				}
				
				return nil
			},
		},
		{
			name:   "Date omission enabled",
			config: ParseConfig{NoDates: true},
			validate: func(nixOutput string, parsedValue Value) error {
				// Date fields should be omitted
				if strings.Contains(nixOutput, "AutoplayPolicyWhitelistConfigurationUpdateDate") {
					return fmt.Errorf("date strings should be omitted when NoDates is true")
				}
				if strings.Contains(nixOutput, "LastVisitTime") {
					return fmt.Errorf("date strings in nested structures should be omitted")
				}
				
				// Other content should still be present
				if !strings.Contains(nixOutput, "AllowJavaScriptFromAppleEvents = true;") {
					return fmt.Errorf("non-date content should be preserved")
				}
				
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nixOutput, parsedValue, err := convertDefaultsWithValueAndConfig(complexInput, tt.config)
			if err != nil {
				t.Fatalf("convertDefaultsWithValueAndConfig failed: %v", err)
			}
			
			if err := tt.validate(nixOutput, parsedValue); err != nil {
				t.Errorf("Validation failed: %v", err)
				t.Logf("Generated Nix output:\n%s", nixOutput)
			}
		})
	}
}

// TestSystemIntegration_SplitModeWorkflow tests the complete split mode workflow
func TestSystemIntegration_SplitModeWorkflow(t *testing.T) {
	// Test the split mode logic with realistic data
	input := `{
    "com.apple.Safari" = {
        HomePage = "https://example.com";
        ExtensionsEnabled = 1;
        TestDate = "2025-06-07 12:01:44 +0000";
    };
    NSGlobalDomain = {
        AppleInterfaceStyle = Dark;
        AppleLanguages = ("en-US", "en");
        AnotherDate = "2024-12-25T10:00:00Z";
    };
    "com.microsoft.VSCode" = {
        AutoUpdateMode = automatic;
        EnableTelemetry = 0;
        FontFamily = "SF Mono";
    };
    "Custom Domain With Spaces" = {
        CustomSetting = "value with spaces";
        NumericSetting = 42;
    };
}`

	tests := []struct {
		name      string
		config    ParseConfig
		expectFiles []string
		validateContent func(bundleID, content string) error
	}{
		{
			name:   "Split with dates preserved",
			config: ParseConfig{NoDates: false},
			expectFiles: []string{
				"com-apple-Safari.nix",
				"NSGlobalDomain.nix", 
				"com-microsoft-VSCode.nix",
				"Custom_Domain_With_Spaces.nix",
			},
			validateContent: func(bundleID, content string) error {
				switch bundleID {
				case "com-apple-Safari":
					if !strings.Contains(content, "HomePage = \"https://example.com\";") {
						return fmt.Errorf("Safari should contain HomePage")
					}
					if !strings.Contains(content, "ExtensionsEnabled = true;") {
						return fmt.Errorf("Safari should convert boolean")
					}
					if !strings.Contains(content, "TestDate = \"2025-06-07 12:01:44 +0000\";") {
						return fmt.Errorf("Safari should preserve dates when NoDates=false")
					}
				case "NSGlobalDomain":
					if !strings.Contains(content, "AppleInterfaceStyle = \"Dark\";") {
						return fmt.Errorf("NSGlobalDomain should contain AppleInterfaceStyle")
					}
					if !strings.Contains(content, "AppleLanguages = [") {
						return fmt.Errorf("NSGlobalDomain should contain AppleLanguages array")
					}
				case "com-microsoft-VSCode":
					if !strings.Contains(content, "EnableTelemetry = false;") {
						return fmt.Errorf("VSCode should convert boolean")
					}
					if !strings.Contains(content, "FontFamily = \"SF Mono\";") {
						return fmt.Errorf("VSCode should handle string with spaces")
					}
				case "Custom_Domain_With_Spaces":
					if !strings.Contains(content, "CustomSetting = \"value with spaces\";") {
						return fmt.Errorf("Custom domain should handle spaced values")
					}
					if !strings.Contains(content, "NumericSetting = 42;") {
						return fmt.Errorf("Custom domain should handle numeric values")
					}
				}
				return nil
			},
		},
		{
			name:   "Split with dates omitted",
			config: ParseConfig{NoDates: true},
			expectFiles: []string{
				"com-apple-Safari.nix",
				"NSGlobalDomain.nix",
				"com-microsoft-VSCode.nix", 
				"Custom_Domain_With_Spaces.nix",
			},
			validateContent: func(bundleID, content string) error {
				// Should not contain any date fields
				if strings.Contains(content, "TestDate") || strings.Contains(content, "AnotherDate") {
					return fmt.Errorf("dates should be omitted from %s when NoDates=true", bundleID)
				}
				
				// Should still contain non-date content
				if bundleID == "com-apple-Safari" && !strings.Contains(content, "HomePage") {
					return fmt.Errorf("non-date content should be preserved in %s", bundleID)
				}
				
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse the input
			_, value, err := convertDefaultsWithValueAndConfig(input, tt.config)
			if err != nil {
				t.Fatalf("Failed to parse input: %v", err)
			}
			
			// Extract bundle IDs
			bundleMap := extractBundleIDs(value)
			if len(bundleMap) != len(tt.expectFiles) {
				t.Errorf("Expected %d bundle IDs, got %d", len(tt.expectFiles), len(bundleMap))
			}
			
			// Validate each expected file
			for _, expectedFile := range tt.expectFiles {
				bundleID := strings.TrimSuffix(expectedFile, ".nix")
				
				// Try both with and without quotes  
				var bundleValue Value
				var found bool
				for key, val := range bundleMap {
					if sanitizeFilename(key) == bundleID {
						bundleValue = val
						found = true
						break
					}
				}
				
				if !found {
					t.Errorf("Expected bundle ID for file %s not found", expectedFile)
					continue
				}
				
				// Generate content and validate
				content := bundleValue.ToNix(0)
				if err := tt.validateContent(bundleID, content); err != nil {
					t.Errorf("Content validation failed for %s: %v", expectedFile, err)
					t.Logf("Generated content for %s:\n%s", expectedFile, content)
				}
			}
		})
	}
}

func TestIsUUIDString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"valid UUID", "A8604994-4D31-471E-B7F1-D60AC97A287C", true},
		{"valid UUID lowercase", "a8604994-4d31-471e-b7f1-d60ac97a287c", true},
		{"valid UUID mixed case", "A8604994-4d31-471E-b7f1-D60AC97A287C", true},
		{"too short", "A8604994-4D31-471E-B7F1", false},
		{"too long", "A8604994-4D31-471E-B7F1-D60AC97A287C-EXTRA", false},
		{"missing hyphens", "A86049944D31471EB7F1D60AC97A287C", false},
		{"wrong hyphen positions", "A860-4994-4D31-471E-B7F1-D60AC97A287C", false},
		{"non-hex characters", "G8604994-4D31-471E-B7F1-D60AC97A287C", false},
		{"empty string", "", false},
		{"not a UUID", "hello-world-this-is-not-a-uuid", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isUUIDString(tt.input)
			if result != tt.expected {
				t.Errorf("isUUIDString(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsHashedIDString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"valid hashed ID", "_19a3bc4999bddb89e1a44f4b87bdc37c", true},
		{"valid hashed ID uppercase", "_19A3BC4999BDDB89E1A44F4B87BDC37C", true},
		{"valid hashed ID mixed", "_fb0549aa0c42c3c83c03adc64ff6c300", true},
		{"no underscore", "19a3bc4999bddb89e1a44f4b87bdc37c", false},
		{"too short", "_19a3bc4999bddb89", false},
		{"too long", "_19a3bc4999bddb89e1a44f4b87bdc37c00", false},
		{"non-hex characters", "_19a3bc4999bddb89e1a44f4b87bdc37g", false},
		{"empty string", "", false},
		{"just underscore", "_", false},
		{"wrong length", "_abc123", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isHashedIDString(tt.input)
			if result != tt.expected {
				t.Errorf("isHashedIDString(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsUUIDKey(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"UUID as key", "A8604994-4D31-471E-B7F1-D60AC97A287C", true},
		{"UUID in key prefix", "001704-05-0990211b-baa3-496b-a477-18acf2584b74-com.apple.systempreferences", true},
		{"UUID in key middle", "prefix-A8604994-4D31-471E-B7F1-D60AC97A287C-suffix", true},
		{"UUID at end", "AccountUUID-3906CAB3-0BD4-41A9-8C1E-80F806043E7D", true},
		{"no UUID", "com.apple.finder", false},
		{"UUID-like but invalid", "not-a-uuid-4D31-471E-B7F1-D60AC97A287C", false},
		{"empty", "", false},
		{"short key", "key", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isUUIDKey(tt.input)
			if result != tt.expected {
				t.Errorf("isUUIDKey(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestUUIDFiltering(t *testing.T) {
	input := `{
		"DeviceID" = "A8604994-4D31-471E-B7F1-D60AC97A287C";
		"Name" = "Test Device";
		"3906CAB3-0BD4-41A9-8C1E-80F806043E7D" = "UUID as key";
		"Regular" = "Value";
		"001704-05-0990211b-baa3-496b-a477-18acf2584b74-com.apple.test" = "Complex UUID key";
		"accountLastKnownUserRecordID" = "_19a3bc4999bddb89e1a44f4b87bdc37c";
		"SHLibraryAvailabilityListenerUserID" = "_fb0549aa0c42c3c83c03adc64ff6c300";
	}`

	// Test without UUID filtering
	result1, err := convertDefaultsWithConfig(strings.NewReader(input), ParseConfig{NoUUIDs: false})
	if err != nil {
		t.Fatalf("Failed to convert without UUID filtering: %v", err)
	}

	// Should contain UUID values and keys
	if !strings.Contains(result1, "DeviceID") {
		t.Error("Expected DeviceID to be present without UUID filtering")
	}
	if !strings.Contains(result1, "3906CAB3-0BD4-41A9-8C1E-80F806043E7D") {
		t.Error("Expected UUID key to be present without UUID filtering")
	}

	// Test with UUID filtering
	result2, err := convertDefaultsWithConfig(strings.NewReader(input), ParseConfig{NoUUIDs: true})
	if err != nil {
		t.Fatalf("Failed to convert with UUID filtering: %v", err)
	}

	// Should not contain UUID values or keys
	if strings.Contains(result2, "DeviceID") {
		t.Error("Expected DeviceID to be filtered out with UUID filtering")
	}
	if strings.Contains(result2, "3906CAB3-0BD4-41A9-8C1E-80F806043E7D") {
		t.Error("Expected UUID key to be filtered out with UUID filtering")
	}
	if strings.Contains(result2, "001704-05-0990211b-baa3-496b-a477-18acf2584b74") {
		t.Error("Expected complex UUID key to be filtered out with UUID filtering")
	}

	// Should not contain hashed IDs
	if strings.Contains(result2, "accountLastKnownUserRecordID") {
		t.Error("Expected accountLastKnownUserRecordID to be filtered out with UUID filtering")
	}
	if strings.Contains(result2, "_19a3bc4999bddb89e1a44f4b87bdc37c") {
		t.Error("Expected hashed ID value to be filtered out with UUID filtering")
	}

	// Should still contain non-UUID values
	if !strings.Contains(result2, "Name") {
		t.Error("Expected Name to be present with UUID filtering")
	}
	if !strings.Contains(result2, "Regular") {
		t.Error("Expected Regular to be present with UUID filtering")
	}
}

func TestIsTimestampKey(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected bool
	}{
		{"CKStartupTime", "CKStartupTime", true},
		{"lastConnected with @", "lastConnected@Display:2", true},
		{"lastUnseen with @", "lastUnseen@Display:7", true},
		{"timestamp in key", "lastAggregatedTimestamp", true},
		{"date in key", "UpdateDate", true},
		{"created in key", "FileCreated", true},
		{"modified in key", "LastModified", true},
		{"expiry in key", "TokenExpiry", true},
		{"regular key", "Username", false},
		{"regular key with at", "Email@domain", false},
		{"Version key", "Version", false},
		{"MixedCase Time", "StartTime", true},
		{"lowercase time", "starttime", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isTimestampKey(tt.key)
			if result != tt.expected {
				t.Errorf("isTimestampKey(%q) = %v, want %v", tt.key, result, tt.expected)
			}
		})
	}
}

func TestTimestampValueDetection(t *testing.T) {
	tests := []struct {
		name     string
		value    float64
		isUnix   bool
		isCF     bool
	}{
		{"Unix timestamp 2025", 1751270386, true, false},
		{"Unix timestamp 2024", 1704067200, true, false},
		{"CFAbsoluteTime 2025", 774728050.470133, false, true},
		{"CFAbsoluteTime 2024", 757382400, false, true},
		{"Small number", 42, false, false},
		{"Large non-timestamp", 9999999999, false, false},
		{"Early CFTime", 100000001, false, true}, // ~2004
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotUnix := isUnixTimestamp(tt.value)
			if gotUnix != tt.isUnix {
				t.Errorf("isUnixTimestamp(%v) = %v, want %v", tt.value, gotUnix, tt.isUnix)
			}
			
			gotCF := isCFAbsoluteTime(tt.value)
			if gotCF != tt.isCF {
				t.Errorf("isCFAbsoluteTime(%v) = %v, want %v", tt.value, gotCF, tt.isCF)
			}
		})
	}
}

func TestTimestampFiltering(t *testing.T) {
	input := `{
		"CKStartupTime" = 1753218075;
		"lastConnected@Display:2" = 774728050.470133;
		"Username" = "testuser";
		"UpdateDate" = "2025-06-07 12:01:44 +0000";
		"Score" = 42;
		"lastAggregatedTimestamp" = 1753142400;
		"RegularField" = 1234567890;
	}`

	// Test without date filtering
	result1, err := convertDefaultsWithConfig(strings.NewReader(input), ParseConfig{NoDates: false})
	if err != nil {
		t.Fatalf("Failed to convert without date filtering: %v", err)
	}

	// Should contain timestamp fields
	if !strings.Contains(result1, "CKStartupTime") {
		t.Error("Expected CKStartupTime to be present without date filtering")
	}
	if !strings.Contains(result1, "lastConnected@Display:2") {
		t.Error("Expected lastConnected to be present without date filtering")
	}

	// Test with date filtering
	result2, err := convertDefaultsWithConfig(strings.NewReader(input), ParseConfig{NoDates: true})
	if err != nil {
		t.Fatalf("Failed to convert with date filtering: %v", err)
	}

	// Should not contain timestamp fields
	if strings.Contains(result2, "CKStartupTime") {
		t.Error("Expected CKStartupTime to be filtered out with date filtering")
	}
	if strings.Contains(result2, "lastConnected@Display:2") {
		t.Error("Expected lastConnected to be filtered out with date filtering")
	}
	if strings.Contains(result2, "lastAggregatedTimestamp") {
		t.Error("Expected lastAggregatedTimestamp to be filtered out with date filtering")
	}
	if strings.Contains(result2, "UpdateDate") {
		t.Error("Expected UpdateDate to be filtered out with date filtering")
	}

	// Should still contain non-timestamp values
	if !strings.Contains(result2, "Username") {
		t.Error("Expected Username to be present with date filtering")
	}
	if !strings.Contains(result2, "Score") {
		t.Error("Expected Score to be present with date filtering")
	}
	// RegularField should be kept even though it looks like a timestamp
	// because the key name doesn't indicate it's a timestamp
	if !strings.Contains(result2, "RegularField") {
		t.Error("Expected RegularField to be present with date filtering")
	}
}
