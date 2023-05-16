{
	channel ? <nixpkgs>,
	attrs ? [],
}:

with builtins;

let pkgs = import channel {};
in

with pkgs.lib;
with builtins;

let pkgs' = attrByPath attrs {} pkgs;

	isValid = x: (tryEval x).success;

	isAttrs = x:
		let eval = tryEval (builtins.isAttrs x);
		in  eval.success && eval.value;

	hasAttr = x: attr:
		let eval = tryEval (builtins.hasAttr attr x);
		in  eval.success && eval.value;

	extractStringAttrs = x:
		filterAttrs (n: v: isString v) x;

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

	relevantMeta = [ "description" "homepage" "license" "url" "version" ];
	filterMeta = x:
		filterAttrs (n: v: elem n relevantMeta) x;

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
		else
			if hasAttr v "meta" && isValid v.meta
			then filterMeta (extractStringAttrs v.meta) // (
				if hasStringAttr v "version"
				then { version = v.version; }
				else { }
			)
			else
				if hasStringAttr v "version"
				then { version = v.version; }
				else { }
	)
	(filterAttrs
		(k: v:
			!(hasPrefix k "_") &&
			(isValid v) &&
			(isAttrs v) &&
			(shouldRecurseInto v || isPackage v))
		(pkgs')
	)
