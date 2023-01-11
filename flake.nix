{
  description = "q";

  inputs = {
    nixpkgs.url = "nixpkgs/nixpkgs-unstable";
    utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, utils }: {
    overlays.default = final: _: {
      q = final.buildGoModule {
        pname = "q";
        version = "0.1.0";
        src = ./.;
        buildInputs =
          if final.stdenv.hostPlatform.isDarwin then
            with final.darwin.apple_sdk_11_0.frameworks;
            [ AppKit AudioToolbox ]
          else [ final.alsa-lib ];
        nativeBuildInputs =
          if final.stdenv.hostPlatform.isDarwin
          then [ ] else [ final.pkg-config ];
        vendorSha256 = null;
      };
    };
  } // utils.lib.eachDefaultSystem (system: with import nixpkgs
    {
      overlays = [
        (final: _: {
          errcheck = final.buildGoModule rec {
            pname = "errcheck";
            version = builtins.substring 0 7 src.rev;
            src = final.fetchFromGitHub {
              owner = "kisielk";
              repo = pname;
              rev = "3faf0bc8574bd40c9635d7910b25dd81349f6e89";
              sha256 = "lx1kbRyL9OJzTxClIej/FisfVRh2VG98HGOBuF359LI=";
            };
            vendorSha256 = "96+927gNuUMovR4Ru/8BwsgEByNq2EPX7wXWS7+kSL8=";
          };
          gow = final.buildGoModule rec {
            pname = "gow";
            version = builtins.substring 0 7 src.rev;
            src = final.fetchFromGitHub {
              owner = "mitranim";
              repo = pname;
              rev = "a5bfab26a0e42ee646f0969ac3397e80e5e3b1df";
              sha256 = "vlIbVoAxeeQ1SB8FmSAfQ35fX6f+/VGZmrPDdA3HTvs=";
            };
            vendorSha256 = "o6KltbjmAN2w9LMeS9oozB0qz9tSMYmdDW3CwUNChzA=";
          };
        })
        self.overlays.default
      ]; inherit system;
    }; {
    packages.default = q;

    devShells = rec {
      ci = (
        if stdenv.hostPlatform.isDarwin
        then mkShell.override { inherit (darwin.apple_sdk_11_0) stdenv; }
        else mkShell
      ) {
        packages = [ go errcheck revive ] ++ (
          if stdenv.hostPlatform.isDarwin then
            with darwin.apple_sdk_11_0.frameworks;
            [ AppKit AudioToolbox ]
          else [ pkg-config alsa-lib ]
        );
      };

      default = ci.overrideAttrs (oldAttrs: {
        nativeBuildInputs = oldAttrs.nativeBuildInputs ++ [ gopls gow ];
      });
    };
  });
}
