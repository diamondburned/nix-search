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

	extractAttrs = x: attrs:
		filterAttrs (n: v: elem n attrs) x;

	tryString = x: attrs:
		if hasAttr x attr
		then toString (getAttr x attr)
		else "";

	isPackage = x:
		# We use tryEval to ensure that the derivation must have a type field with the string
		# "derivation".
		x ? type && x ? outPath && x.type == "derivation";

	shouldRecurseInto = x:
		isAttrs x &&
		x ? recurseForDerivations &&
		x.recurseForDerivations == true;

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
		if isPackage v
		then
			v.meta // (
				if v ? version
				then { version = v.version; }
				else
					if (v ? meta && v.meta ? version)
					then { version = v.meta.version; }
					else { }
			)
		else
			{ hasMore = true; })
	(filterAttrs
		(k: v:
			!(hasPrefix k "_") &&
			(isValid v) &&
			(isAttrs v) &&
			(isPackage v || shouldRecurseInto v))
		(pkgs'))
