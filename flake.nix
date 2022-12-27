{
  description = "q";

  inputs = {
    nixpkgs.url = "nixpkgs/nixpkgs-unstable";
    utils.url = "github:numtide/flake-utils";
    errcheck-src = {
      url = "github:kisielk/errcheck";
      flake = false;
    };
    gow-src = {
      url = "github:mitranim/gow";
      flake = false;
    };
  };

  outputs = { self, nixpkgs, utils, errcheck-src, gow-src }: {
    overlays.default = final: _: {
      q = final.buildGoModule {
        pname = "q";
        version = builtins.readFile ./internal/version/version.txt;
        src = ./.;
        buildInputs = [ final.alsa-lib ];
        nativeBuildInputs = [ final.pkg-config ];
        vendorSha256 = null;
      };
    };
  } // utils.lib.eachDefaultSystem (system: with import nixpkgs
    {
      overlays = [
        (final: _: {
          errcheck = final.buildGoModule {
            pname = "errcheck";
            version = errcheck-src.shortRev;
            src = errcheck-src;
            vendorSha256 = "96+927gNuUMovR4Ru/8BwsgEByNq2EPX7wXWS7+kSL8=";
          };
          gow = final.buildGoModule {
            pname = "gow";
            version = gow-src.shortRev;
            src = gow-src;
            vendorSha256 = "o6KltbjmAN2w9LMeS9oozB0qz9tSMYmdDW3CwUNChzA=";
          };
        })
        self.overlays.default
      ]; inherit system;
    }; {
    packages.default = q;

    devShells.default = mkShell {
      packages = [ go gopls gow errcheck revive pkg-config alsa-lib ];
    };
  });
}
