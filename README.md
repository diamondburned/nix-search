# nix-search

A better and channel-compatible `nix search` for NixOS using only stable Nix
tools.

## Why?

I cannot get Flakes to work, no one was able to help me for days, so I decided
to [fix the Nix channels issues
myself](https://github.com/diamondburned/nix-bonito).

Unstable Nix versions (2.3+) break all of `nix`'s subcommands including `nix
search`, so I decided to make a better `nix search` that works with stable Nix
versions.

## Goals

The goals of `nix-search` is to be fast and useful. Nix 2.3's `nix search` is
rather weak and slow in its searching capabilities, so this aims to be better
than that.

`nix-search` will go with an indexing-based approach, where it will index
packages and their attributes into a searching database. This will allow for
faster searching and more accurate results. [Bleve](https://blevesearch.com/)
will be used for the indexing.

`nix-search` will also eventually feature a lightweight expression evaluator
that will allow for more accurate results. This will allow for more flexible
queries, such as `nix-search -e 'description contains "foo" and name contains
"bar"'`.

### Non-goals

Flakes are not supported. I don't use them. Also, the Nix developers should
actually develop a better `nix search`, since nothing in Nix is stable anyway :)

## Usage

TODO
