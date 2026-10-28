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
        # The release version: scripts/release.sh check compares the tag with it.
        version = "0.1.1";
        catalog = pkgs.buildGoModule {
          pname = "catalog";
          inherit version;
          src = self;
          subPackages = [ "tools/cmd/catalog" ];
          # Change it when go.sum changes: nix build prints the new value.
          vendorHash = "sha256-uwgLBHwWDDKv7ya4nwp3VBfDdXD+++evvmqA5co0qDQ=";
          doCheck = false;
        };
        # L0 runs this API server with etcd and no nodes. Keep the version equal to the RKE2
        # release that the Zeabur installer pins.
        kube-apiserver = pkgs.buildGoModule rec {
          pname = "kube-apiserver";
          version = "1.36.5";
          src = pkgs.fetchFromGitHub {
            owner = "kubernetes";
            repo = "kubernetes";
            rev = "v${version}";
            hash = "sha256-M5wVu5678Eow6xMLNZXcfPLlS9z11RpwwzvmYd8NEeE=";
          };
          vendorHash = null;
          subPackages = [ "cmd/kube-apiserver" ];
          ldflags = [
            "-s"
            "-w"
            "-X k8s.io/component-base/version.gitVersion=v${version}"
            "-X k8s.io/component-base/version.gitMajor=1"
            "-X k8s.io/component-base/version.gitMinor=36"
            "-X k8s.io/client-go/pkg/version.gitVersion=v${version}"
          ];
          doCheck = false;
        };
      in {
        packages = {
          inherit catalog kube-apiserver;
          # The files that scripts/release.sh signs and publishes. Zeabur builds this output at its
          # pinned commit, so it embeds the same bytes as the release.
          release = pkgs.runCommand "catalog-release" { nativeBuildInputs = [ catalog ]; } ''
            cp -r ${self} src
            chmod -R u+w src
            cd src
            catalog index -out index.json
            catalog crosscheck -out crosscheck.json
            tar --sort=name --mtime=@0 --owner=0 --group=0 --numeric-owner --mode=u+w,go-w,a+rX \
              -c index.json crosscheck.json components templates schema config measurements LICENSE README.md |
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
            kube-apiserver
            etcd
            cosign
            gh
            jq
            shellcheck
            actionlint
            yq-go
          ];
        };
      });
}
