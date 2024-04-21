{
	inputs = {
		nixpkgs.url = "github:nixos/nixpkgs?ref=nixos-unstable";
		flake-utils.url = "github:numtide/flake-utils";
	};

	outputs = { self, nixpkgs, flake-utils }: flake-utils.lib.eachDefaultSystem (system:
		let
			pkgs = nixpkgs.legacyPackages.${system};
		in
		{
			devShells.default = pkgs.mkShell {
				packages = with pkgs; [
					go
					gopls
					gotools
					sqlc
				];
			};

			packages.default = pkgs.buildGoModule {
				pname = "nix-search";
				version = self.rev or "unknown";
				src = self;

				vendorHash = "sha256-h4NVsdJksMkVai1fPc6nV9SouD3DK4UMYl70G9JJw4g=";

				meta = with pkgs.lib; {
					description = "A better and channel-compatible `nix search` for NixOS using only stable Nix tools.";
					homepage = "https://libdb.so/nix-search";
					mainProgram = "nix-search";
				};
			};

			apps = rec {
				default = nix-search;
				nix-search = {
					type = "app";
					program = "${self.packages.${system}.default}/bin/nix-search";
				};
				nix-dump-index = {
					type = "app";
					program = "${self.packages.${system}.default}/bin/nix-dump-index";
				};
			};
		}
	);
}
