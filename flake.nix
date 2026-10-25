{
  description = "Zeabur Catalog: tested Kubernetes components and application templates, with their schema and tools";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        catalog = pkgs.buildGoModule {
          pname = "catalog";
          version = "0";
          src = self;
          subPackages = [ "tools/cmd/catalog" ];
          # Change it when go.sum changes: nix build prints the new value.
          vendorHash = "sha256-3BY8xtnl4+1muLx8+4ZHYFW3dT/ZP1/3d2dn9yOofV8=";
          doCheck = false;
        };
      in {
        packages = {
          inherit catalog;
          # The files that release.yml signs and publishes. Zeabur builds this output at its
          # pinned commit, so it embeds the same bytes as the release.
          release = pkgs.runCommand "catalog-release" { nativeBuildInputs = [ catalog ]; } ''
            cp -r ${self} src
            chmod -R u+w src
            cd src
            catalog index -out index.json
            tar --sort=name --mtime=@0 --owner=0 --group=0 --numeric-owner --mode=u+w,go-w,a+rX \
              -c index.json components templates schema config measurements LICENSE README.md |
              gzip -9n > bundle.tar.gz
            sha256sum bundle.tar.gz index.json > checksums.txt
            mkdir $out
            cp bundle.tar.gz index.json checksums.txt $out/
          '';
        };

        devShells.default = pkgs.mkShell {
          packages = with pkgs; [
            go
            gopls
            kubernetes-helm
            kubectl
            kubeconform
            kind
            cosign
            jq
            yq-go
          ];
        };
      });
}
