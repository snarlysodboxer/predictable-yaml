{
  description = "Lint and fix YAML key ordering";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs =
    {
      self,
      nixpkgs,
      flake-utils,
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        predictable-yaml-configs = pkgs.fetchFromGitHub {
          owner = "snarlysodboxer";
          repo = "predictable-yaml-configs";
          rev = "v1.0.5";
          hash = "sha256-WGeqbhaw+jDx81DsWQAnzp6tpMP8PqswqGY+lUI7qj8=";
        };
        predictable-yaml = pkgs.buildGoModule {
          pname = "predictable-yaml";
          version = "v0.3.0";
          src = ./.;
          vendorHash = "sha256-Q6NvALG9KKsepYlUJ+wOmDVIfhQmOlYvr68uxkQ6pZ0=";
          env.CGO_ENABLED = "0";
          ldflags = [
            "-X github.com/snarlysodboxer/predictable-yaml/cmd.Version=${predictable-yaml.version}"
          ];
          nativeBuildInputs = [ pkgs.installShellFiles ];
          preBuild = ''
            cp ${predictable-yaml-configs}/*.yaml internal/embedded/configs/ || true
            cp ${predictable-yaml-configs}/*.yml internal/embedded/configs/ || true
          '';
          postInstall = ''
            installShellCompletion --cmd predictable-yaml \
              --bash <($out/bin/predictable-yaml completion bash) \
              --zsh <($out/bin/predictable-yaml completion zsh) \
              --fish <($out/bin/predictable-yaml completion fish)
          '';
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
    )
    // {
      overlays.default = final: prev: {
        predictable-yaml = self.packages.${prev.system}.predictable-yaml;
      };
    };
}
