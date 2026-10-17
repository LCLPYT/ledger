{
  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs =
    {
      self,
      nixpkgs,
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
      devShells = forEachSupportedSystem (
        {
          system,
          pkgs,
        }:
        {
          default = pkgs.mkShell {
            packages = with pkgs; [
              # API tooling
              protoc-gen-go
              protoc-gen-go-grpc
              api-linter
              grpcui
              grpcurl

              jdk25
            ];

            hardeningDisable = [ "fortify" ];

            JAVA_HOME = "${pkgs.jdk25.home}";

            # consistent location for SDK selection in IDEs
            shellHook = ''
              ln -sfn ${pkgs.jdk25.home} .jdk
            '';
          };
        }
      );
    };
}
