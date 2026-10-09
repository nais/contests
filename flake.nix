{
  inputs.flake-utils.url = "github:numtide/flake-utils";
  inputs.nixpkgs.url = "github:nixos/nixpkgs/nixpkgs-unstable";

  outputs =
    inputs:
    inputs.flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = import inputs.nixpkgs { localSystem = { inherit system; }; };
      in
      {
        devShells.default = pkgs.mkShellNoCC {
          packages = with pkgs; [
            go_latest
          ];
        };
      }
    );
}
