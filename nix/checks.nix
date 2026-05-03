{ inputs, ... }:
{
  perSystem =
    { pkgs, self', ... }:
    let
      kghPackage = pkgs.callPackage ./kgh.nix {
        self = inputs.self;
      };
    in
    {
      checks = {
        package = self'.packages.default;

        test = kghPackage.overrideAttrs (old: {
          pname = "${old.pname}-test";
          doCheck = true;
          nativeCheckInputs = (old.nativeCheckInputs or [ ]) ++ [ pkgs.git ];
          checkPhase = ''
            runHook preCheck
            go test ./...
            runHook postCheck
          '';
        });

        vet = kghPackage.overrideAttrs (old: {
          pname = "${old.pname}-vet";
          doCheck = true;
          nativeCheckInputs = (old.nativeCheckInputs or [ ]) ++ [ pkgs.git ];
          checkPhase = ''
            runHook preCheck
            go vet ./...
            runHook postCheck
          '';
        });

        fmt = pkgs.runCommand "kgh-gofmt-check" { nativeBuildInputs = [ pkgs.go ]; } ''
          cd ${inputs.self}
          if [ -n "$(gofmt -l .)" ]; then
            echo "gofmt found unformatted files" >&2
            gofmt -l .
            exit 1
          fi
          mkdir -p "$out"
        '';
      };
    };
}
