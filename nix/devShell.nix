{ ... }:
{
  perSystem = { pkgs, ... }: {
    devShells.default = pkgs.mkShell {
      packages = with pkgs; [
        go
        gopls
        gotools
        git
        python3
        python3Packages.kaggle
      ];

      shellHook = ''
        echo "kgh Nix dev shell"
        echo "Available: go, gopls, python3, kaggle"
      '';
    };
  };
}
