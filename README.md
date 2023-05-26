# nix-search

A better and channel-compatible `nix search` for NixOS using only stable Nix
tools.

## Why?

I cannot get Flakes to work, no one was able to help me for days, so I decided
to [fix the Nix channels issues
myself](https://github.com/diamondburned/nix-bonito).

Unstable Nix versions (2.4+) break all of `nix`'s subcommands including `nix
search`, so I decided to make a better `nix search` that works with stable Nix
versions.

## Goals

The goals of `nix-search` is to be fast and useful. Nix 2.3's `nix search` is
rather weak and slow in its searching capabilities, and Nix 2.4's `nix search`
is even slower, so this aims to be better than that.

`nix-search` goes with an indexing-based approach, where it indexes packages
and their attributes into a searching database. This allows for faster
searching and more accurate results.
[Bluge](https://github.com/blugelabs/bluge) is used for the indexing.

`nix-search` will also eventually feature a lightweight expression evaluator
that will allow for more accurate results. This will allow for more flexible
queries, such as `nix-search -e 'description contains "foo" and name contains
"bar"'`.

### Non-goals

Flakes are not supported. I don't use them. Also, the Nix developers should
actually develop a better `nix search`, since nothing in Nix is stable anyway :)

## Installation

### Nix

TODO: make `default.nix`

### Go

```sh
go install libdb.so/nix-search/cmd/nix-search@latest
```

## Usage

First, index the Nixpkgs tree:

```sh
nix-search --index
```

Then, search for packages:

```sh
nix-search firefox
```

## Performance

`nix-search` is reasonably fast. It takes about 20 seconds to index the entire
Nixpkgs tree, and searching is almost instantaneous.

```
―❤―▶ time nix-search --index

real	0m21.760s
user	2m36.436s
sys	0m30.729s

―❤―▶ time nix-search firefox > /dev/null

real	0m0.033s
user	0m0.028s
sys	0m0.006s

```
