{
  inputs = {
    flake-parts.url = "github:hercules-ci/flake-parts";
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };
  outputs = inputs @ {flake-parts, ...}:
    flake-parts.lib.mkFlake {inherit inputs;} ({...}: {
      systems = ["x86_64-linux" "aarch64-linux" "aarch64-darwin" "x86_64-darwin"];
      imports = [inputs.flake-parts.flakeModules.easyOverlay];
      perSystem = {pkgs, ...}: let
        mod = builtins.fromJSON (builtins.readFile ./pkg.json);
        pkg =
          pkgs.buildGoModule
          {
            src = ./.;
            pname = mod.name;
            meta.mainprogram = mod.name;
            version = mod.version;
            hash = mod.bin-hash;
            vendorHash = null;
            doCheck = false;
          };
      in {
        packages = {
          default = pkg;
        };
        devShells.default = pkgs.mkShell {
          packages = [pkg];
        };
      };
    });
}
