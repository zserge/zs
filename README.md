zs
==

[![Build Status](https://travis-ci.org/zserge/zs.svg?branch=master)](https://travis-ci.org/zserge/zs)

zs is an extremely minimal static site generator written in Go.

It's inspired by `zas` generator, but is even more minimal.

The name stands for 'zen static' as well as it's my initials.

## Features

* Zero configuration (no configuration file needed)
* Cross-platform
* Highly extensible
* Works well for blogs and generic static websites (landing pages etc)
* Easy to learn
* Fast

## Installation

Download the binaries from Github or build it manually:

	$ go get github.com/zserge/zs

## Ideology

Keep your texts in markdown, [amber] or HTML format right in the main directory
of your blog/site.

Keep all service files (extensions, layout pages, deployment scripts etc)
in the `.zs` subdirectory.

Define variables in the header of the content files using [YAML]:

	title: My web site
	keywords: best website, hello, world
	---

	Markdown text goes after a header *separator*

Use placeholders for variables and plugins in your markdown or html
files, e.g. `{{ title }}` or `{{ command arg1 arg2 }}.

Write extensions in any language you like and put them into the `.zs`
subdiretory.

Everything the extensions prints to stdout becomes the value of the
placeholder.

Every variable from the content header will be passed via environment variables like `title` becomes `$ZS_TITLE` and so on. There are some special variables:

* `$ZS` - a path to the `zs` executable
* `$ZS_OUTDIR` - a path to the directory with generated files
* `$ZS_FILE` - a path to the currently processed markdown file
* `$ZS_URL` - a URL for the currently generated page

## Example of RSS generation

Extensions can be written in any language you know (Bash, Python, Lua, JavaScript, Go, even Assembler). Here's an example of how to scan all markdown blog posts and create RSS items:

``` bash
for f in ./blog/*.md ; do
	d=$($ZS var $f date)
	if [ ! -z $d ] ; then
		timestamp=`date --date "$d" +%s`
		url=`$ZS var $f url`
		title=`$ZS var $f title | tr A-Z a-z`
		descr=`$ZS var $f description`
		echo $timestamp \
			"<item>" \
			"<title>$title</title>" \
			"<link>http://zserge.com/$url</link>" \
			"<description>$descr</description>" \
			"<pubDate>$(date --date @$timestamp -R)</pubDate>" \
			"<guid>http://zserge.com/$url</guid>" \
		"</item>"
	fi
done | sort -r -n | cut -d' ' -f2-
```

## Hooks

There are two special plugin names that are executed every time the build
happens - `prehook` and `posthook`. You can define some global actions here like
content generation, or additional commands, like LESS to CSS conversion:

	# .zs/post

	#!/bin/sh
	lessc < $ZS_OUTDIR/styles.less > $ZS_OUTDIR/styles.css
	rm -f $ZS_OUTDIR/styles.less

## Syntax sugar

By default, `zs` converts each `.amber` file into `.html`, so you can use lightweight Jade-like syntax instead of bloated HTML.

Also, `zs` converts `.gcss` into `.css`, so you don't really need LESS or SASS. More about GCSS can be found [here][gcss].

## Command line usage

`zs build` re-builds your site.

`zs build <file>` re-builds one file and prints resulting content to stdout.

`zs watch` rebuilds your site every time you modify any file.

`zs var <filename> [var1 var2...]` prints a list of variables defined in the
header of a given markdown file, or the values of certain variables (even if
it's an empty string).

## License

The software is distributed under the MIT license.

[amber]: https://github.com/eknkc/amber/
[YAML]: https://github.com/go-yaml/yaml
[gcss]: https://github.com/yosssi/gcss
