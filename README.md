# `defaults2nix`

A Go utility to convert macOS `defaults` output to Nix.

This tool makes it easy to convert macOS system preferences and application settings into declarative Nix configurations for use with nix-darwin or Home Manager.

## Features

- Convert macOS defaults to Nix attribute sets
- Support for all standard data types (booleans, numbers, strings, arrays, dictionaries)
- Automatic binary data filtering (skips non-useful binary entries)
- Proper string escaping and quoting
- Preserve nested structures and key ordering
- Flexible filtering with `-filter` flag (dates, state, uuids)

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
  -all       Process all defaults from `defaults read`
  -filter    Comma-separated list of items to filter out (dates,state,uuids)
  -split     Split defaults into individual Nix files by domain
  -o, -out   Output file or directory path

Arguments:
  domain     The domain to convert (e.g., com.apple.dock)

Examples:
  defaults2nix com.apple.Safari
  defaults2nix com.apple.Safari -o safari.nix
  defaults2nix -all -o all-defaults.nix
  defaults2nix -all -filter dates -o all-defaults.nix
  defaults2nix -all -filter state,uuids -o all-defaults.nix
  defaults2nix -split -o ./configs/
  sudo defaults2nix -all -o all-defaults.nix  # for system configs
```

### Filtering Options

The `-filter` flag accepts a comma-separated list of items to filter out from the output:

```bash
# Filter out date values
defaults2nix com.apple.Safari -filter dates -o safari.nix

# Filter out UI state and UUIDs
defaults2nix -all -filter state,uuids -o clean-defaults.nix

# Filter out all supported types
defaults2nix -all -filter dates,state,uuids -o minimal-defaults.nix

# Works with split mode too
defaults2nix -split -filter dates,state -o ./configs/
```

> **⚠️ Warning**: The filtering mechanisms are based on heuristics and pattern matching, which means they may occasionally:
> - Filter out legitimate configuration values that happen to look like timestamps, UUIDs, or UI state
> - Miss some values that should be filtered if they don't match expected patterns
> - Be overly aggressive with timestamp filtering based on key names
> 
> Always review the filtered output to ensure important settings haven't been inadvertently removed. These filters are intended as a convenience for creating cleaner configurations, not as a precise data classification system.

#### Available Filters:

- **dates**: Omits timestamp values
  - String dates: `2025-06-07 12:01:44 +0000`, `2025-06-07T12:01:44Z`, `2025-06-07`
  - Unix timestamps: Integer values in timestamp-related keys (e.g., `CKStartupTime = 1753218075`)
  - CFAbsoluteTime: Floating-point values in timestamp-related keys (e.g., `lastConnected@Display:2 = 774728050.470133`)
  - Detects timestamps based on key names containing: time, date, timestamp, created, modified, updated, etc.

- **state**: Omits UI state and window geometry
  - Window frame positions and sizes
  - Split view configurations
  - Toolbar customizations
  - Table view settings

- **uuids**: Omits UUID keys and values
  - Standard UUIDs: `A8604994-4D31-471E-B7F1-D60AC97A287C`
  - Hashed identifiers: `_19a3bc4999bddb89e1a44f4b87bdc37c` (underscore + 32 hex chars)
  - Keys containing UUID patterns
  - Helps create more reproducible configurations

### Split Domains into Separate Files

The `-split` flag processes all available domains and creates individual `.nix` files for each:

```bash
# Creates separate files for each domain in the specified directory
defaults2nix -split -out ./configs/

# For complete coverage including system-level configs
sudo defaults2nix -split -out ./configs/
```

This will create individual `.nix` files for each domain found:
- `com.apple.Safari.nix`
- `com.apple.finder.nix` 
- `NSGlobalDomain.nix`
- `loginwindow.nix`
- etc.

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
| `"2025-06-07 12:01:44 +0000"` | *skipped with -filter dates* | Date values can be filtered |
| `"A8604994-4D31-471E-B7F1-D60AC97A287C"` | *skipped with -filter uuids* | UUID values can be filtered |
| `NSWindow Frame ...` | *skipped with -filter state* | UI state can be filtered |

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
