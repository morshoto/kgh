{ inputs, ... }:
{
  perSystem = { pkgs, ... }: {
    packages.default = pkgs.callPackage ./kgh.nix {
      self = inputs.self;
    };
  };
}
