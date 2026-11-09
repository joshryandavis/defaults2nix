# `defaults2nix`

Convert macOS `defaults` output to Nix, for nix-darwin and Home Manager.

## Install

```bash
nix run github:joshryandavis/defaults2nix -- com.apple.dock
nix profile install github:joshryandavis/defaults2nix
```

For system-level settings, run with `sudo`.

## Usage

```
defaults2nix [flags] [domain]
```

Flags must come before the domain.

```bash
# One domain, to stdout or a file
defaults2nix com.apple.Safari
defaults2nix -out safari.nix com.apple.Safari

# Every domain from `defaults read`
defaults2nix -all -out all.nix

# One file per domain
defaults2nix -split -out ./configs/

# Drop dates, UI state and UUIDs
defaults2nix -all -filter dates,state,uuids -out clean.nix
```

`-o` is an alias for `-out`.

| Flag | What it does |
| --- | --- |
| `-all` | Convert every domain `defaults read` returns |
| `-split` | Write one Nix file per domain into `-out` |
| `-out`, `-o` | Output file, or directory with `-split` |
| `-filter` | Comma-separated values to drop: `dates`, `state`, `uuids` |

`-split` names files after the domain, replacing dots with dashes and spaces with
underscores: `com.apple.Safari` becomes `com-apple-Safari.nix`.

## Filters

Filters are heuristics. They aim at values that make a config noisy, and they can
drop something you wanted, so read the output.

- **dates**: keys whose name mentions a time, plus values in
  `2026-11-06 14:42:34 +0000`, `2026-11-06T14:42:34Z` or `2026-11-06` form.
- **state**: window frames, split view geometry, toolbar layout, caches.
- **uuids**: UUID values and keys, and `_` + 32 hex digit identifiers.

Binary data (`{length = N, bytes = 0x...}`) is always dropped.

## Input and output

```
{
    AutoFillCreditCardData = 1;
    HomePage = "https://www.apple.com/startpage/";
    FrequentlyVisitedSites = (
        {
            Title = "Example Site";
            Score = "42.5";
        },
        "Simple String Item"
    );
}
```

```nix
{
  AutoFillCreditCardData = true;
  HomePage = "https://www.apple.com/startpage/";
  FrequentlyVisitedSites = [
    {
      Title = "Example Site";
      Score = 42.5;
    }
    "Simple String Item"
  ];
}
```

`defaults` prints booleans as `1` and `0`, and numbers as plain decimals. A number
becomes a Nix number; anything that is not a number becomes a string. Keys that
Nix cannot take bare are quoted.

## Use it

```nix
# nix-darwin
system.defaults.CustomUserPreferences."com.apple.Safari" = import ./configs/com-apple-Safari.nix;

# Home Manager
targets.darwin.defaults."com.apple.Safari" = import ./configs/com-apple-Safari.nix;
```

## Development

```bash
nix develop -c go test ./...
nix build
```

The flake takes its version from `version.txt`; refresh the inputs with
`nix flake update`.

`nix build` reads the git tree, so new files must be staged to be built.

## Limitations

`defaults` prints its own text format, and some information is not in it: a string
`"1"` and a number `1` look the same, and floats like `1.0` arrive as a bare `1`, so
they become booleans. Non-ASCII text arrives as `\U` escapes and is decoded. Binary
blobs are dropped, and unusual types may need a look before you use them.
