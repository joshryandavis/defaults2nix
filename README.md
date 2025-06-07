# defaults2nix

A Go utility to convert macOS `defaults` output to Nix configuration format.

This is just a proof-of-concept. I've not gone too deeply into how macOS `defaults` or Apple plists work.

It just makes copying defaults into my flake config easier.

## Installation

### Using Nix

The easiest way to install is using Nix:

```bash
# Run directly from the flake
nix run github:joshryandavis/defaults2nix

# Or install to your profile
nix profile install github:joshryandavis/defaults2nix
```

## Usage

### From file
```bash
defaults read com.apple.Safari > safari.plist
defaults2nix safari.plist > safari.nix
```

### From stdin
```bash
defaults read com.apple.Safari | defaults2nix
```

## Input Format

The tool expects the standard output format from macOS `defaults read` commands, which is a property list (plist) format. For example:

```
{
    AutoFillCreditCardData = 1;
    AutoOpenSafeDownloads = 0;
    HomePage = "https://www.apple.com/startpage/";
    ExtensionsEnabled = 1;
    FrequentlyVisitedSites = (
        {
            Title = "Example Site";
            URL = "https://example.com/";
        }
    );
}
```

## Output Format

The tool converts the input to Nix attribute set syntax:

```nix
{
  AutoFillCreditCardData = true;
  AutoOpenSafeDownloads = false;
  HomePage = "https://www.apple.com/startpage/";
  ExtensionsEnabled = true;
  FrequentlyVisitedSites = [
    {
      Title = "Example Site";
      URL = "https://example.com/";
    }
  ];
}
```

## Type Conversions

- `1` → `true`
- `0` → `false`
- Numbers are preserved as-is
- Strings are properly quoted and escaped
- Arrays `()` are converted to lists `[]`
- Dictionaries `{}` are converted to attribute sets
- Binary data values with `length` and `bytes` are skipped (not useful in Nix)

## Common Use Cases

### Safari Configuration
```bash
defaults read com.apple.Safari | defaults2nix
```

### Finder Configuration
```bash
defaults read com.apple.finder | defaults2nix
```

### System Preferences
```bash
defaults read NSGlobalDomain | defaults2nix
```

## Limitations

- Complex nested data structures are supported but may require manual review
- Some macOS-specific data types may need additional handling
- Binary data entries are automatically skipped as they're not useful in declarative configurations
