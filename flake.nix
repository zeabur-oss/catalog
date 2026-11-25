{
  description = "Zeabur Catalog: tested Kubernetes components and application templates, with their schema and tools";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
    # The nightly of rust-toolchain.toml.
    rust-overlay = {
      url = "github:oxalica/rust-overlay";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    # The base shell of every zeabur-oss repository: Bun, the release tools and the shared linters.
    oss = {
      url = "github:zeabur-oss/.github/0558836e9cf7e00101c99d4fd123276e4a20fe20";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs =
    {
      self,
      nixpkgs,
      flake-utils,
      rust-overlay,
      oss,
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = import nixpkgs {
          inherit system;
          overlays = [ rust-overlay.overlays.default ];
        };
        # The nightly, components and targets of rust-toolchain.toml, the same file rustup reads.
        rustToolchain = pkgs.rust-bin.fromRustupToolchainFile ./rust-toolchain.toml;
        # The release version: the Release workflow compares the tag with it.
        version = "0.6.3";
        catalog = pkgs.buildGoModule {
          pname = "catalog";
          inherit version;
          src = self;
          subPackages = [ "tools/cmd/catalog" ];
          # Change it when go.sum changes: nix build prints the new value. On Renovate's pull
          # requests the vendorhash workflow commits it.
          vendorHash = "sha256-vkEk/5ykLAidEcWL/9VSV/i2l7sC1aDpnjcZnE4WE5c=";
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
          # The bundle that a release publishes. Zeabur OSS builds this output at its
          # pinned commit, so it embeds the same bytes as the release. The static API needs helm
          # to fetch charts, so scripts/ci/release.sh builds it outside this sandbox.
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
            rustToolchain
            cargo-deny
            cargo-nextest
            kubectl
            kubeconform
            kind
            kube-apiserver
            etcd
            victoriametrics
            yq-go
            # Pattern rules (scripts/ci/patterns.sh)
            ripgrep
          ];
          RUST_SRC_PATH = "${rustToolchain}/lib/rustlib/src/rust/library";
        };
      }
    );
}
