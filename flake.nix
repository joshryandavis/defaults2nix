{
  inputs = {
    flake-parts.url = "github:hercules-ci/flake-parts";
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };
  outputs = inputs @ {flake-parts, ...}:
    flake-parts.lib.mkFlake {inherit inputs;} ({...}: {
      systems = ["x86_64-linux" "aarch64-linux" "aarch64-darwin" "x86_64-darwin"];
      perSystem = {pkgs, ...}: let
        pkg = pkgs.buildGoModule {
          src = ./.;
          pname = "defaults2nix";
          version = pkgs.lib.trim (builtins.readFile ./version.txt);
          vendorHash = null;
          doCheck = true;
          meta = {
            description = "Convert macOS defaults to Nix";
            license = pkgs.lib.licenses.mit;
            mainProgram = "defaults2nix";
          };
        };
      in {
        packages.default = pkg;
        devShells.default = pkgs.mkShell {
          inputsFrom = [pkg];
          packages = [pkg];
        };
      };
    });
}
