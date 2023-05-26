let overlay = self: super: 
	let nixpkgs_go_1_20 = import (super.fetchFromGitHub {
			owner  = "NixOS";
			repo   = "nixpkgs";
			rev    = "46c194bd83efc43da64cb137cab1178071f09c3b";
			sha256 = "01gjm68kr373fznbhgzbkblm1c7ry5mfvsdh4qpvy0h0wd7m8fsw";
		}) {};	
	in
	{
		go =
			if super.lib.versionAtLeast super.go.version "1.20"
			then super.go
			else self.go_1_20;
		go_1_20 =
			if super ? go_1_20
			then super.go_1_20
			else nixpkgs_go_1_20.go_1_20;
		buildGoModule = super.buildGoModule.override {
			go = self.go;
		};
	};

in { pkgs ? import <nixpkgs> {} }:

let
	lib = pkgs.lib;

	pkgs_go_1_20 = import <nixpkgs> { overlays = [ overlay ]; };

	sqlc = pkgs_go_1_20.buildGoModule rec {
		name = "sqlc";
		version = "1.17.2";
		src = pkgs.fetchFromGitHub {
			repo = "sqlc";
			owner = "kyleconroy";
			rev = "v${version}";
			sha256 = "sha256-30dIFo07C+noWdnq2sL1pEQZzTR4FfaV0FvyW4BxCU8=";
		};
		vendorSha256 = "0ih9siizn6nkvm4wi0474wxy323hklkhmdl52pql0qzqanmri4yb";
		doCheck = false;
		proxyVendor = true;
		subPackages = [ "cmd/sqlc" ];
	};

in pkgs.mkShell {
	buildInputs =
		(with pkgs_go_1_20; [
			go
			gopls
			gotools
			sqlc
		]);
}
