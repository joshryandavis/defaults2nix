#!/usr/bin/env -S nix shell nixpkgs#bash nixpkgs#go nixpkgs#jq --impure --command bash

set -xeuo pipefail
cd "$(dirname "$0")"

nix flake update
VERSION=$(cat version.txt)
TMP_BIN=$(mktemp)
trap 'rm -f "$TMP_BIN"' EXIT
go mod tidy && go build -o "$TMP_BIN"
BIN_HASH=$(nix hash file "$TMP_BIN")
jq -n \
  --arg bin_hash "$BIN_HASH" \
  --arg version "$VERSION" \
  --arg name "defaults2nix" \
  '{
     "name": $name,
     "version": $version,
     "bin-hash": $bin_hash
  }' \
  >pkg.json
