{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    git-hooks = {
      url = "github:cachix/git-hooks.nix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs =
    {
      self,
      nixpkgs,
      git-hooks,
    }:
    let
      supportedSystems = [
        "x86_64-linux"
        "aarch64-linux"
        "aarch64-darwin"
      ];
      forEachSupportedSystem =
        f:
        nixpkgs.lib.genAttrs supportedSystems (
          system:
          f {
            pkgs = import nixpkgs { inherit system; };
            inherit system;
          }
        );
    in
    {
      checks = forEachSupportedSystem (
        { system, ... }:
        {
          git-hooks = git-hooks.lib.${system}.run {
            src = ./.;
            hooks.convco.enable = true;
          };
        }
      );

      devShells = forEachSupportedSystem (
        {
          system,
          pkgs,
        }:
        let
          hooks = self.checks.${system}.git-hooks;
        in
        {
          default = pkgs.mkShell {
            packages = hooks.enabledPackages ++ (with pkgs; [
              # API tooling
              protoc-gen-go
              protoc-gen-go-grpc
              api-linter
              grpcui
              grpcurl

              jdk25
            ]);

            hardeningDisable = [ "fortify" ];

            JAVA_HOME = "${pkgs.jdk25.home}";

            # consistent location for SDK selection in IDEs
            shellHook = hooks.shellHook + ''
              ln -sfn ${pkgs.jdk25.home} .jdk
            '';
          };
        }
      );
    };
}
