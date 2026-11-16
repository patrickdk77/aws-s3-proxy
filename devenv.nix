{
  pkgs,
  lib,
  config,
  inputs,
  ...
}:

{
  packages = [
    pkgs.docker
    pkgs.gh
    pkgs.gh-actions-lock
    pkgs.go
  ];

  # https://devenv.sh/languages/
  languages.python = {
    enable = true;
    package = pkgs.python3.withPackages (ps: [
      ps.boto3
      ps.httpx2
      ps.jwcrypto
      ps.moto
      ps.pytest
    ] ++ ps.moto.optional-dependencies.server);
    venv.enable = true;
  };

  # https://devenv.sh/git-hooks/
  git-hooks.hooks.golangci-lint.enable = true;
  git-hooks.hooks.ruff.enable = true;
  git-hooks.hooks.ruff-format.enable = true;

  # See full reference at https://devenv.sh/reference/options/
}
