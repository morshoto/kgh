{
  lib,
  buildGoModule,
  self,
}:
buildGoModule (finalAttrs: {
  pname = "kgh";
  version =
    if self ? rev
    then self.shortRev
    else "dev";

  src = lib.cleanSource self;
  vendorHash = "sha256-g+yaVIx4jxpAQ/+WrGKxhVeliYx7nLQe/zsGpxV4Fn4=";

  subPackages = [ "cmd/kgh" ];
  doCheck = false;

  ldflags = [
    "-s"
    "-w"
    "-X"
    "main.version=${finalAttrs.version}"
  ];

  env.CGO_ENABLED = 0;

  meta = {
    description = "GitHub-native tool for Kaggle workflows";
    homepage = "https://github.com/shotomorisk/kgh";
    license = lib.licenses.mit;
    mainProgram = "kgh";
  };
})
