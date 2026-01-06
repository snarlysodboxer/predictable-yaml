{
  description = "Lint and fix YAML key ordering";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        predictable-yaml = pkgs.buildGoModule {
          pname = "predictable-yaml";
          version = "0.1.0";
          src = ./.;
          vendorHash = "sha256-BqMayrlLSgOx4tuAl2vyQnUjLm7WizfMxdNc/ku+KGk=";
          env.CGO_ENABLED = "0";
          meta = with pkgs.lib; {
            description = "Lint and fix YAML key ordering";
            homepage = "https://github.com/snarlysodboxer/predictable-yaml";
            license = licenses.asl20;
            mainProgram = "predictable-yaml";
          };
        };
      in
      {
        packages = {
          default = predictable-yaml;
          predictable-yaml = predictable-yaml;
        };

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            go
            gopls
            delve
          ];
        };
      }
    ) // {
      overlays.default = final: prev: {
        predictable-yaml = self.packages.${prev.system}.predictable-yaml;
      };
    };
}
