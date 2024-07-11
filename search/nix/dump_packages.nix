{
	nixpkgs ? <nixpkgs>,
	system ? builtins.currentSystem,
	attrs ? [],
}:

with builtins;

let
	pkgs = import nixpkgs {
		inherit system;
	};
in

with pkgs.lib;
with builtins;

let
	pkgs' = attrByPath attrs {} pkgs;

	isValid = x: (tryEval x).success;

	isAttrs = x:
		let eval = tryEval (builtins.isAttrs x);
		in  eval.success && eval.value;

	hasAttr = x: attr:
		let eval = tryEval (builtins.hasAttr attr x);
		in  eval.success && eval.value;

	hasStringAttr = x: attr:
		hasAttr x attr &&
		isValid x.${attr} &&
		isString x.${attr};

	isPackage = x:
		hasAttr x "type" &&
		hasAttr x "outPath" &&
		x.type == "derivation";

	isString = x:
		let eval = tryEval (builtins.isString x);
		in  eval.success && eval.value;

	shouldRecurseInto = x:
		isAttrs x &&
		hasAttr x "recurseForDerivations"	&&
		x.recurseForDerivations == true;

	licenseString = license:
		if isString license
		then license
		else
			if isAttrs license && license ? "spdxId"
			then license.spdxId
			else null;

	filterPackageMeta = pkg:
		# List of meta attributes to include in the output.
		# Keep this in sync with [search.Package].
		(filterAttrs (n: v: elem n [
			"version"
			"description"
			"longDescription"
			"mainProgram"
			"broken"
		]) pkg.meta) // rec {
			licenses =
				if pkg.meta ? "license"
				then map licenseString (singleton pkg.meta.license)
				else null;
			unfree =
				if pkg.meta ? "license" && isValid pkg.meta.license
				then any
					(license: license ? "free" && !license.free)
					(singleton pkg.meta.license)
				else false;
			unsupportedPlatform =
				let
					availableOn = tryEval (meta.availableOn pkgs pkg);
				in
					!availableOn.success || !availableOn.value;

			# homepages =
			# 	if pkg.meta ? "homepage"
			# 	then singleton pkg.meta.homepage
			# 	else null;
		};

	# bfs is too slow for Nix.
	# bfs = pkgs: mapAttrs
	# 	(k: v:
	# 		if (!isAttrs v || isPackage v)
	# 		then null
	# 		else bfs v)
	# 	(pkgs);
in

mapAttrs
	(k: v:
		if shouldRecurseInto v
		then { hasMore = true; }
		else { meta =
			if hasAttr v "meta" && isValid v.meta
			then filterPackageMeta v // (
				if hasStringAttr v "version"
				then { version = v.version; }
				else { }
			)
			else
				if hasStringAttr v "version"
				then { version = v.version; }
				else { };
		}
	)
	(filterAttrs
		(k: v:
			!(hasPrefix k "_") &&
			(isValid v) &&
			(isAttrs v) &&
			(shouldRecurseInto v || isPackage v))
		(pkgs')
	)
