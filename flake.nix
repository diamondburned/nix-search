{
	inputs = {
		nixpkgs.url = "github:nixos/nixpkgs?ref=nixos-unstable";
		flake-utils.url = "github:numtide/flake-utils";
		flake-compat.url = "https://flakehub.com/f/edolstra/flake-compat/1.tar.gz";
	};

	outputs = { self, nixpkgs, flake-utils, ... }: flake-utils.lib.eachDefaultSystem (system:
		let
			pkgs = nixpkgs.legacyPackages.${system};
		in
		{
			devShells.default = pkgs.mkShell {
				packages = with pkgs; [
					go_1_23
					gopls
					gotools
					sqlc
				];
			};

			packages.default = pkgs.buildGoModule {
				pname = "nix-search";
				version = self.rev or "unknown";
				src = self;

				vendorHash = "sha256-bModWDH5Htl5rZthtk/UTw/PXT+LrgyBjsvE6hgIePY=";

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
