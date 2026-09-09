{
  inputs.nixpkgs-unstable.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs =
    {
      self,
      nixpkgs-unstable,
    }:
    let
      supportedSystems = [ "x86_64-linux" ];
      forEachSupportedSystem =
        f:
        nixpkgs-unstable.lib.genAttrs supportedSystems (
          system:
          f {
            pkgs-unstable = import nixpkgs-unstable { inherit system; };
            inherit system;
          }
        );
    in
    {
      devShells = forEachSupportedSystem (
        {
          system,
          pkgs-unstable,
        }:
        {
          default = pkgs-unstable.mkShell {
            packages = with pkgs-unstable; [
              # API tooling
              protoc-gen-go
              protoc-gen-go-grpc
              api-linter
              grpcui
              grpcurl
            ];

            hardeningDisable = [ "fortify" ];
          };
        }
      );
    };
}