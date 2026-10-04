{
  description = "Nix flake for tsuzuri";

  inputs = {
    # Using the unstable nixpkgs repository
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    utils.url = "github:numtide/flake-utils";
  };

  outputs = {
    self,
    nixpkgs,
    utils,
  }:
    utils.lib.eachDefaultSystem (system: let
      pkgs = import nixpkgs {inherit system;};
    in {
      packages.default = pkgs.buildGoModule {
        pname = "tsuzuri";
        version = "0.1.0";

        src = ./.;

        vendorHash = "sha256-Al9YpToJRYq71aupVq+mn8yYSb1GefHZcz3Dz+mCgVU=";

        meta = {
          description = "tsuzuri";
          homepage = "https://github.com/jaisuriya-11/tsuzuri/tree/main";
        };
      };
      apps.default = utils.lib.mkApp {
        drv = self.packages.${system}.default;
      };
    });
}
