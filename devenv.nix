{ pkgs, lib, config, inputs, ... }:

let
  nydus-app = pkgs.buildGoModule {
    pname = "nydus";
    version = "0.1.0";
    src = ./.;
    vendorHash = null;
    # CGO required for go-sqlite3
    nativeBuildInputs = [ pkgs.gcc ];
    buildInputs = [ pkgs.sqlite ];
    env.CGO_ENABLED = "1";
    subPackages = [ "cmd/nydus" ];
  };

  nydus-runtime = pkgs.buildEnv {
    name = "nydus-runtime";
    paths = [ nydus-app pkgs.cacert pkgs.sqlite pkgs.bash ];
    pathsToLink = [ "/bin" "/etc" "/lib" ];
  };
in
{
  languages.go.enable = true;

  packages = with pkgs; [
    buf
    protobuf
    sqlite
    gcc
    cacert
    inputs.nix2container.packages.${pkgs.system}.skopeo-nix2container
  ];

  processes = {
    nydus.exec = "go run ./cmd/nydus";
  };

  containers.nydus = {
    name = "nydus";
    copyToRoot = [ nydus-runtime ];
    startupCommand = "${nydus-app}/bin/nydus";
  };

  tasks = {
    "ci:build" = {
      exec = "go build -o nydus ./cmd/nydus";
    };
    "ci:test" = {
      exec = "go test ./...";
    };
    "container:build" = {
      exec = "devenv container build nydus";
    };
    "container:copy" = {
      exec = ''
        IMAGE=$(devenv container build nydus 2>&1 | tail -1)
        nix run github:nlewo/nix2container#skopeo-nix2container -- copy nix:$IMAGE containers-storage:nydus:latest
        echo "Container copied to podman: nydus:latest"
      '';
    };
    "container:run" = {
      exec = "podman run --rm -d --name nydus -p 15318:15318 -e NYDUS_DB_PATH=/data/nydus.db -v nydus-data:/data nydus:latest";
    };
    "container:stop" = {
      exec = "podman stop nydus && podman rm nydus";
    };
  };

  enterShell = ''
    echo "Nydus Development Environment"
    echo "Commands: go run ./cmd/nydus | go test ./... | go build ./cmd/nydus"
    echo ""
    echo "Container commands:"
    echo "  devenv task container:build   - Build OCI container"
    echo "  devenv task container:copy    - Copy to podman"
    echo "  devenv task container:run     - Run container (port 15318)"
  '';
}
