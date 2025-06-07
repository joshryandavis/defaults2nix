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
				"key with spaces": StringValue{Value: "value"},
				"key-with-dashes": StringValue{Value: "value2"},
			},
			[]string{"key with spaces", "key-with-dashes"},
			"{\n  \"key with spaces\" = \"value\";\n  \"key-with-dashes\" = \"value2\";\n}",
		},
		{
			"Dict with skip value",
			map[string]Value{
				"normalKey": StringValue{Value: "value"},
				"binaryKey": SkipValue{},
			},
			[]string{"normalKey", "binaryKey"},
			"{\n  normalKey = \"value\";\n}",
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
		{
			"String value",
			"hello",
			StringValue{Value: "hello"},
		},
		{
			"Quoted string",
			"\"hello world\"",
			StringValue{Value: "hello world"},
		},
		{
			"Simple array",
			"(item1, item2)",
			ArrayValue{Values: []Value{
				StringValue{Value: "item1"},
				StringValue{Value: "item2"},
			}},
		},
		{
			"Simple dict",
			"{key1 = value1; key2 = value2;}",
			DictValue{
				Values: map[string]Value{
					"key1": StringValue{Value: "value1"},
					"key2": StringValue{Value: "value2"},
				},
				Order: []string{"key1", "key2"},
			},
		},
		{
			"Binary data (should return SkipValue)",
			"{length = 256, bytes = 0x89504e47 0d0a1a0a;}",
			SkipValue{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseValue(tt.input)

			switch expected := tt.expected.(type) {
			case StringValue:
				if sv, ok := result.(StringValue); ok {
					if sv.Value != expected.Value {
						t.Errorf("parseValue() StringValue = %q, want %q", sv.Value, expected.Value)
					}
				} else {
					t.Errorf("parseValue() type = %T, want StringValue", result)
				}
			case SkipValue:
				if _, ok := result.(SkipValue); !ok {
					t.Errorf("parseValue() type = %T, want SkipValue", result)
				}
			case ArrayValue:
				if av, ok := result.(ArrayValue); ok {
					if len(av.Values) != len(expected.Values) {
						t.Errorf("parseValue() ArrayValue length = %d, want %d", len(av.Values), len(expected.Values))
					}
				} else {
					t.Errorf("parseValue() type = %T, want ArrayValue", result)
				}
			case DictValue:
				if dv, ok := result.(DictValue); ok {
					if len(dv.Values) != len(expected.Values) {
						t.Errorf("parseValue() DictValue length = %d, want %d", len(dv.Values), len(expected.Values))
					}
				} else {
					t.Errorf("parseValue() type = %T, want DictValue", result)
				}
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

	expected := `{
  AllowJavaScriptFromAppleEvents = true;
  AutoFillCreditCardData = true;
  AutoOpenSafeDownloads = false;
  ShowStandaloneTabBar = false;
  HomePage = "https://www.apple.com/startpage/";
  ExtensionsEnabled = true;
}`

	result, err := convertDefaults(strings.NewReader(input))
	if err != nil {
		t.Fatalf("convertDefaults() error = %v", err)
	}

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

	// Binary data should be skipped
	if strings.Contains(result, "BinaryData") {
		t.Error("convertDefaults() should skip binary data, but BinaryData found in result")
	}
	if strings.Contains(result, "MoreBinaryData") {
		t.Error("convertDefaults() should skip binary data, but MoreBinaryData found in result")
	}

	// Other settings should be present
	if !strings.Contains(result, "TestSetting = true") {
		t.Error("convertDefaults() should include TestSetting")
	}
	if !strings.Contains(result, "HomePage = \"https://example.com\"") {
		t.Error("convertDefaults() should include HomePage")
	}
	if !strings.Contains(result, "AnotherSetting = \"value\"") {
		t.Error("convertDefaults() should include AnotherSetting")
	}
	if !strings.Contains(result, "LastSetting = false") {
		t.Error("convertDefaults() should include LastSetting")
	}
}

func TestConvertDefaults_ComplexSafariExample(t *testing.T) {
	input := `{
    AllowJavaScriptFromAppleEvents = 1;
    AutoFillCreditCardData = 1;
    ExtensionsEnabled = 1;
    "ExtensionsToolbarConfiguration BrowserStandaloneTabBarToolbarIdentifier-v2" = {
        OrderedToolbarItemIdentifiers = (
            CombinedSidebarTabGroupToolbarIdentifier,
            SidebarSeparatorToolbarItemIdentifier,
            BackForwardToolbarIdentifier
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
    customizationSyncServerToken = {length = 293, bytes = 0x62706c69 73743030 d4010203 04050607};
}`

	result, err := convertDefaults(strings.NewReader(input))
	if err != nil {
		t.Fatalf("convertDefaults() error = %v", err)
	}

	// Check that basic values are converted correctly
	if !strings.Contains(result, "AllowJavaScriptFromAppleEvents = true") {
		t.Error("convertDefaults() should convert 1 to true")
	}

	// Check that quoted keys are handled
	if !strings.Contains(result, "\"ExtensionsToolbarConfiguration BrowserStandaloneTabBarToolbarIdentifier-v2\"") {
		t.Error("convertDefaults() should preserve quoted keys")
	}

	// Check that arrays are converted
	if !strings.Contains(result, "OrderedToolbarItemIdentifiers = [") {
		t.Error("convertDefaults() should convert arrays")
	}

	// Check that nested dictionaries work
	if !strings.Contains(result, "FrequentlyVisitedSitesCache = [") {
		t.Error("convertDefaults() should handle array of dictionaries")
	}

	// Check that binary data is skipped
	if strings.Contains(result, "customizationSyncServerToken") {
		t.Error("convertDefaults() should skip binary data")
	}
}

func TestParseArrayElements(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []Value
	}{
		{
			"Simple elements",
			"item1, item2, item3",
			[]Value{
				StringValue{Value: "item1"},
				StringValue{Value: "item2"},
				StringValue{Value: "item3"},
			},
		},
		{
			"Elements with semicolons",
			"item1;, item2;, item3;",
			[]Value{
				StringValue{Value: "item1"},
				StringValue{Value: "item2"},
				StringValue{Value: "item3"},
			},
		},
		{
			"Quoted elements",
			"\"item 1\", \"item 2\"",
			[]Value{
				StringValue{Value: "item 1"},
				StringValue{Value: "item 2"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseArrayElements(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("parseArrayElements() length = %d, want %d", len(result), len(tt.expected))
				return
			}

			for i, expected := range tt.expected {
				if sv, ok := result[i].(StringValue); ok {
					if expectedSv, ok := expected.(StringValue); ok {
						if sv.Value != expectedSv.Value {
							t.Errorf("parseArrayElements()[%d] = %q, want %q", i, sv.Value, expectedSv.Value)
						}
					}
				}
			}
		})
	}
}

func TestSkipValue_ToNix(t *testing.T) {
	sv := SkipValue{}
	result := sv.ToNix(0)
	if result != "" {
		t.Errorf("SkipValue.ToNix() = %q, want empty string", result)
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
