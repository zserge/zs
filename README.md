zs
==

zs is an extremely minimal static site generator written in Go.

It's inspired by `zas` generator, but is even more minimal.

The name stands for 'zen static' as well as it's my initials.

## Features

* Zero configuration (no configuration file needed)
* Cross-platform
* Highly extensible
* Easy to learn
* Fast

## Installation

Download the binaries from Github or build it manually:

	$ go get github.com/zserge/zs

## Ideology

Keep your texts in markdown format in the root directory of your blog/site.

Keep all service files (extensions, layout pages, deployment scripts etc)
in the `.zs` subdirectory.

Define variables in the header of the markdown files:

	title: My web site
	keywords: best website, hello, world

	Markdown text goes after a *newline*

Use placeholders for variables and plugins in your markdown or html
files, e.g. `{{ title }}`.

Write extensions in any language you like and put them into the `.zs`
subdiretory.

Everything the extensions prints to stdout becomes the value of the
placeholder.

Extensions can use special environment variables, like:

* `$ZS` - a path to the `zs` executable
* `$ZS_OUTDIR` - a path to the directory with generated files
* `$ZS_FILE` - a path to the currently processed markdown file
* `$ZS_URL` - a URL for the currently generated page

You can also pass command line arguments, e.g: `{{ my-plugin arg1 arg2 }}`

## Example of RSS generation

## Hooks

There are two special plugin names that are executed every time the build
happens - `pre` and `post`. You can define some global action here like compile
your LESS to CSS etc:

	# .zs/post

	#!/bin/sh
	lessc < $ZS_OUTDIR/styles.less > $ZS_OUTDIR/styles.css
	rm -f $ZS_OUTDIR/styles.css

## Command line usage

`zs build` re-builds your site.

`zs watch` rebuilds your site every time you modify any file.

`zs var <filename> [var1 var2...]` prints a list of variables defined in the
header of a given markdown file, or the values of certain variables (even if
it's an empty string).

## License

The software is distributed under the MIT license.
