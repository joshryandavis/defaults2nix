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
        info = builtins.fromJSON (builtins.readFile ./nixpkg.json);
        pkg =
          pkgs.buildGoModule
          {
            src = ./.;
            pname = info.name;
            meta.mainprogram = info.name;
            version = info.version;
            hash = info.bin-hash;
            vendorHash = null;
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
