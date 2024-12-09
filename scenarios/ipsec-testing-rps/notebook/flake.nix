{
  description = "Dev Shell for Python";
  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  inputs.flake-utils.url = "github:numtide/flake-utils";
  inputs.devshell.url = "github:numtide/devshell";

  outputs = { nixpkgs, flake-utils, devshell, ... }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs {
          inherit system;
          overlays = [ 
            devshell.overlays.default
          ];
        };
        packages = with pkgs; [
          (python3.withPackages (ps: with ps; [
            virtualenv
            pip
            setuptools
            wheel
          ]))
        ];
      in
      {
        devShells.default = pkgs.devshell.mkShell rec {
          name = "Python";
          inherit packages;
          env = [
            {
              name = "LD_LIBRARY_PATH";
              value = pkgs.lib.makeLibraryPath [ pkgs.stdenv.cc.cc ];
            }
          ];
          devshell.startup."setprompt" = pkgs.lib.noDepEntry ''
            export LP_MARK_PREFIX=" (python) "
          '';
          devshell.startup."printpackages" = pkgs.lib.noDepEntry ''
            echo "[[ Packages ]]"
            echo "${builtins.concatStringsSep "\n" (builtins.map (p: p.name) packages)}"
            echo ""
          '';
        };
      }
    );
}
