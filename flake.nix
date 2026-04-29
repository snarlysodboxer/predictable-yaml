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
          rev = "v1.0.3";
          hash = "sha256-2FJOzXISFwZ30XG8BUeWR4NmrPuXbUFuld2UZQ5UxF8=";
        };
        predictable-yaml = pkgs.buildGoModule {
          pname = "predictable-yaml";
          version = "v0.2.1";
          src = ./.;
          vendorHash = "sha256-BqMayrlLSgOx4tuAl2vyQnUjLm7WizfMxdNc/ku+KGk=";
          env.CGO_ENABLED = "0";
          ldflags = [ "-X github.com/snarlysodboxer/predictable-yaml/cmd.Version=${predictable-yaml.version}" ];
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
