package main

import (
	"fmt"
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
