{
  description = "Zeabur Catalog: tested Kubernetes components and application templates, with their schema and tools";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let pkgs = nixpkgs.legacyPackages.${system};
      in {
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
