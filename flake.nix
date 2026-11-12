{
  description = "Zeabur Catalog: tested Kubernetes components and application templates, with their schema and tools";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
    # The base shell of every zeabur-oss repository: Bun, the release tools and the shared linters.
    oss = {
      url = "github:zeabur-oss/.github";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs =
    {
      self,
      nixpkgs,
      flake-utils,
      oss,
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        # The release version: scripts/release.ts check compares the tag with it.
        version = "0.5.0";
        catalog = pkgs.buildGoModule {
          pname = "catalog";
          inherit version;
          src = self;
          subPackages = [ "tools/cmd/catalog" ];
          # Change it when go.sum changes: nix build prints the new value. On Renovate's pull
          # requests the vendorhash workflow commits it.
          vendorHash = "sha256-pcFPYUbTe0cSOgj7mo1F7/UojQczgcQwDzyOPf65EiY=";
          doCheck = false;
        };
        # The check tier runs this API server with etcd and no nodes. Keep the version equal to the RKE2
        # release in config/kubernetes.yaml.
        kube-apiserver = pkgs.buildGoModule rec {
          pname = "kube-apiserver";
          version = "1.37.2";
          src = pkgs.fetchFromGitHub {
            owner = "kubernetes";
            repo = "kubernetes";
            rev = "v${version}";
            hash = "sha256-Bbyu5hMQTSrvr/kDFHlRnfz5RIR7ecBfTFOGTCffn5c=";
          };
          vendorHash = null;
          subPackages = [ "cmd/kube-apiserver" ];
          ldflags = [
            "-s"
            "-w"
            "-X k8s.io/component-base/version.gitVersion=v${version}"
            "-X k8s.io/component-base/version.gitMajor=1"
            "-X k8s.io/component-base/version.gitMinor=37"
            "-X k8s.io/client-go/pkg/version.gitVersion=v${version}"
          ];
          doCheck = false;
        };
      in
      {
        packages = {
          inherit catalog kube-apiserver;
          # The bundle that scripts/release.ts publishes. Zeabur builds this output at its
          # pinned commit, so it embeds the same bytes as the release. The static API needs helm
          # to fetch charts, so release.ts builds it outside this sandbox.
          release = pkgs.runCommand "catalog-release" { nativeBuildInputs = [ catalog ]; } ''
            cp -r ${self} src
            chmod -R u+w src
            cd src
            # Git keeps no empty directory, and the tar below needs measurements/.
            mkdir -p measurements
            catalog crosscheck -out crosscheck.json
            tar --sort=name --mtime=@0 --owner=0 --group=0 --numeric-owner --mode=u+w,go-w,a+rX \
              -c crosscheck.json components templates schema config conformance measurements LICENSE README.md |
              gzip -9n > bundle.tar.gz
            mkdir $out
            cp bundle.tar.gz $out/
          '';
        };

        devShells.default = pkgs.mkShell {
          inputsFrom = [ oss.devShells.${system}.base ];
          packages = with pkgs; [
            go
            gopls
            kubectl
            kubeconform
            kind
            kube-apiserver
            etcd
            victoriametrics
            yq-go
          ];
        };
      }
    );
}
