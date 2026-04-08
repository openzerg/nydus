{
  description = "Nydus - OpenZerg pub/sub messaging service";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-25.11";
  };

  outputs = { self, nixpkgs }:
    let
      system = "x86_64-linux";
      pkgs = nixpkgs.legacyPackages.${system};
    in
    {
      devShells.${system}.default = pkgs.mkShell {
        buildInputs = with pkgs; [
          go_1_25
          gcc
          sqlite.dev
          git
          gnumake
        ];

        CGO_ENABLED = "1";
        shellHook = ''
          echo "nydus dev shell: go $(go version)"
        '';
      };

      packages.${system}.default = pkgs.buildGo125Module {
        pname = "nydus";
        version = "0.1.0";
        src = ./.;
        ldflags = [ "-s" "-w" ];
        CGO_ENABLED = "1";
        buildInputs = with pkgs; [ sqlite.dev ];
        nativeBuildInputs = with pkgs; [ gcc ];
      };
    };
}
