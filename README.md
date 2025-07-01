# `defaults2nix`

A Go utility to convert macOS `defaults` output to Nix.

This tool makes it easy to convert macOS system preferences and application settings into declarative Nix configurations for use with nix-darwin or Home Manager.

## Features

- Convert macOS defaults to Nix attribute sets
- Support for all standard data types (booleans, numbers, strings, arrays, dictionaries)
- Automatic binary data filtering (skips non-useful binary entries)
- Proper string escaping and quoting
- Preserve nested structures and key ordering

## Installation

### Using Nix

The easiest way to install is using Nix:

```bash
# Run directly from the flake
nix run github:joshryandavis/defaults2nix

# Or install to your profile
nix profile install github:joshryandavis/defaults2nix
```

### Building from Source

### Basic Usage

#### From file
```bash
git clone https://github.com/joshryandavis/defaults2nix
cd defaults2nix
go build -o defaults2nix
```

## Usage

> **Note**: For system-level configurations and certain global settings, you may need to run commands with `sudo` to access all preferences.

### Basic Usage - Single Domain

Convert defaults for a specific domain:

```bash
# Output to stdout
defaults2nix com.apple.Safari

# Save to file
defaults2nix com.apple.Safari -o safari.nix
```

### All Domains

Convert all system defaults at once:

```bash
# Output to stdout (may need sudo for system-level configs)
defaults2nix -all

# Save to file
defaults2nix -all -o all-defaults.nix

# For complete system settings, run as root
sudo defaults2nix -all -o all-defaults.nix
```

### Split by Domain

Convert all domains into separate files:

```bash
# Creates individual .nix files for each domain
defaults2nix -split -o ./nix-configs/

# For system-level configs, run as root
sudo defaults2nix -split -o ./nix-configs/

# This creates files like:
# ./nix-configs/com.apple.Safari.nix
# ./nix-configs/com.apple.finder.nix
# ./nix-configs/NSGlobalDomain.nix
# etc.
```

### Command Line Options

```
Usage: defaults2nix [flags] [domain]

A tool for converting macOS defaults into Nix templates.

Flags:
  -all      Process all defaults from `defaults read`
  -split    Split defaults into individual Nix files by domain
  -o, -out  Output file or directory path

Arguments:
  domain     The domain to convert (e.g., com.apple.dock)

Examples:
  defaults2nix com.apple.Safari
  defaults2nix com.apple.Safari -o safari.nix
  defaults2nix -all -o all-defaults.nix
  defaults2nix -split -o ./configs/
  sudo defaults2nix -all -o all-defaults.nix  # for system configs
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

# For system-level preferences
sudo defaults export | defaults2nix -split -output ./configs
```

This will create individual `.nix` files for each top-level key found in the input:
- `com-apple-Safari.nix`
- `NSGlobalDomain.nix`
- `Custom_User_Preferences.nix`
- `loginwindow.nix`
- etc.

### Command Line Options

- `-split`: Split top-level keys into individual files
- `-output <dir>`: Output directory for split files (default: current directory)
- `-help`: Show usage information

## Input Format

The tool processes the standard output format from macOS `defaults read` commands:

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
            Score = "42.5";
        },
        "Simple String Item"
    );
}
```

## Output Format

Converts to clean Nix attribute set syntax:

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
      Score = 42.5;
    }
    "Simple String Item"
  ];
}
```

## Type Conversions

| macOS Format | Nix Format | Notes |
|--------------|------------|-------|
| `1` | `true` | Boolean true |
| `0` | `false` | Boolean false |
| `42` | `42` | Integers preserved |
| `3.14` | `3.14` | Floats preserved |
| `"string"` | `"string"` | Quoted strings |
| `simple` | `"simple"` | Unquoted identifiers become strings |
| `(item1, item2)` | `[item1 item2]` | Arrays to lists |
| `{key = value;}` | `{key = value;}` | Dictionaries to attribute sets |
| `{length = N; bytes = 0x...}` | *skipped* | Binary data filtered out |

## Key Handling

The tool automatically handles special cases for Nix attribute names:

- Numeric keys: `123` → `"123"`
- Keys with spaces: `key name` → `"key name"`
- Keys with dashes: `key-name` → `"key-name"`
- Keys starting with numbers: `1password` → `"1password"`
- Nix keywords: `with`, `let`, `in`, etc. → quoted
- Keys with dots: `com.apple.Safari` → `"com.apple.Safari"`

## Common Use Cases

### Single Application Configuration
```bash
# Get Safari settings
defaults2nix com.apple.Safari -o safari.nix

# Use in nix-darwin
system.defaults.CustomUserPreferences."com.apple.Safari" = import ./safari.nix;
```

### Multiple Applications and Global Settings (Split Mode)
```bash
# Get Finder settings
defaults2nix com.apple.finder -o finder.nix

# Use in Home Manager
targets.darwin.defaults."com.apple.finder" = import ./finder.nix;
```

### System-wide Settings
```bash
# Get global domain settings (may require sudo)
defaults2nix NSGlobalDomain -o global.nix

# For complete system settings
sudo defaults2nix NSGlobalDomain -o global.nix

# Use in nix-darwin
system.defaults.NSGlobalDomain = import ./global.nix;
```

### Complete System Migration
```bash
# Export all current settings (run as root for complete coverage)
sudo defaults2nix -split -o ./my-macos-config/

# Then selectively import the ones you want in your Nix configuration
```

## Integration Examples

### With nix-darwin

```nix
# configuration.nix
{ config, pkgs, ... }:

{
  system.defaults = {
    # Import converted settings
    NSGlobalDomain = import ./defaults/NSGlobalDomain.nix;

    CustomUserPreferences = {
      "com.apple.Safari" = import ./defaults/com.apple.Safari.nix;
      "com.apple.finder" = import ./defaults/com.apple.finder.nix;
    };
  };
}
```

### With Home Manager

```nix
# home.nix
{ config, pkgs, ... }:

{
  targets.darwin.defaults = {
    "com.apple.Safari" = import ./defaults/com.apple.Safari.nix;
    "com.apple.finder" = import ./defaults/com.apple.finder.nix;
  };
}
```

## Limitations

- This is a proof-of-concept tool focused on common use cases
- Some exotic macOS data types may need manual review
- Binary data is automatically skipped (by design)
- Complex custom objects may require additional handling
- System-level configurations may require root access to read completely
