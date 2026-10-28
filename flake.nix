{
  description = "socialstats: a tool for agents that collects YouTube and X creator analytics into local files";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs =
    { self, nixpkgs }:
    let
      system = "x86_64-linux";
      pkgs = nixpkgs.legacyPackages.${system};
      version = "0.1.0";
      socialstats = pkgs.buildGoModule {
        pname = "socialstats";
        inherit version;
        src = pkgs.lib.cleanSource ./.;
        vendorHash = "sha256-9ckbTepa8WyKX4ln5DET69x6dY09aLUxQVf7wokwKJw=";
        ldflags = [
          "-s"
          "-w"
          "-X socialstats/internal/config.Version=${version}"
        ];
        meta = {
          description = "Collect your own YouTube and X creator analytics into local JSON/CSV for agents";
          license = pkgs.lib.licenses.mit;
          mainProgram = "socialstats";
        };
      };
    in
    {
      packages.${system} = {
        default = socialstats;
        socialstats = socialstats;
      };

      apps.${system}.default = {
        type = "app";
        program = "${socialstats}/bin/socialstats";
      };

      homeManagerModules.default =
        { pkgs, lib, ... }:
        {
          imports = [ ./nix/hm-module.nix ];
          programs.socialstats.package = lib.mkDefault (
            self.packages.${pkgs.stdenv.hostPlatform.system}.default or socialstats
          );
        };

      devShells.${system}.default = pkgs.mkShell {
        packages = [
          pkgs.go
          pkgs.gopls
        ];
      };

      checks.${system} = {
        socialstats = socialstats;
      };

      formatter.${system} = pkgs.nixfmt;
    };
}
