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

### Basic Usage

#### From file
```bash
defaults read com.apple.Safari > safari.plist
defaults2nix safari.plist > safari.nix
```

#### From stdin
```bash
defaults read com.apple.Safari | defaults2nix
```

### Split Top-Level Keys

You can split all top-level keys (bundle IDs, NSGlobalDomain, custom preferences, etc.) into individual files using the `-split` flag:

#### Split from file
```bash
defaults export > all-defaults.plist
defaults2nix -split -output ./configs all-defaults.plist
```

#### Split from stdin
```bash
defaults export | defaults2nix -split -output ./configs
```

This will create individual `.nix` files for each top-level key found in the input:
- `com-apple-Safari.nix`
- `com-google-Chrome.nix` 
- `com-microsoft-VSCode.nix`
- `NSGlobalDomain.nix`
- `Custom_User_Preferences.nix`
- `loginwindow.nix`
- etc.

### Command Line Options

- `-split`: Split top-level keys into individual files
- `-output <dir>`: Output directory for split files (default: current directory)
- `-help`: Show usage information

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

### Single Application Configuration
```bash
defaults read com.apple.Safari | defaults2nix
```

### Multiple Applications and Global Settings (Split Mode)
```bash
# Export all defaults for your user
defaults export | defaults2nix -split -output ~/nix-configs

# This creates individual files like:
# ~/nix-configs/com-apple-Safari.nix
# ~/nix-configs/com-apple-finder.nix
# ~/nix-configs/com-apple-dock.nix
# ~/nix-configs/NSGlobalDomain.nix
# ~/nix-configs/loginwindow.nix
# ~/nix-configs/Custom_User_Preferences.nix
```

### System Preferences
```bash
defaults read NSGlobalDomain | defaults2nix
```

## Limitations

- Complex nested data structures are supported but may require manual review
- Some macOS-specific data types may need additional handling
- Binary data entries are automatically skipped as they're not useful in declarative configurations
