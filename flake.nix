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
        src = builtins.path { path = ./.; name = "q-src"; };
        buildInputs =
          if final.stdenv.hostPlatform.isDarwin then
            with final.darwin.apple_sdk_11_0.frameworks;
            [ AppKit AudioToolbox ]
          else [ final.alsa-lib ];
        nativeBuildInputs =
          if final.stdenv.hostPlatform.isDarwin
          then [ ] else [ final.pkg-config ];
        vendorHash = null;
      };
    };
  } // utils.lib.eachDefaultSystem (system: with import nixpkgs
    { overlays = [ self.overlays.default ]; inherit system; }; {
    packages.default = q;

    devShells = rec {
      ci = (
        if stdenv.hostPlatform.isDarwin
        then mkShell.override { inherit (darwin.apple_sdk_11_0) stdenv; }
        else mkShell
      ) {
        packages = [ go ] ++ (
          if stdenv.hostPlatform.isDarwin then
            with darwin.apple_sdk_11_0.frameworks;
            [ AppKit AudioToolbox ]
          else [ pkg-config alsa-lib ]
        );
      };

      default = ci.overrideAttrs (oldAttrs: {
        nativeBuildInputs = oldAttrs.nativeBuildInputs ++ [ gopls ];
      });
    };
  });
}
