# A fork of a fork of Google Code Search 

    From junkblocker's fork of the original - though still somewhat of an experiment to see how well it works

    Changes include:

    - Fixed windows mapping handling so indices over 1Gb work
    - Removed an unused copy of trigram from index
    - Simple run length encoding of deltas of 1 to reduce post size
    - Don't store root directory on every file name but reference the entry in the list of root directories
    - Concurrent grepping on files that are selected from the index

## To install this fork

    go get -u github.com/waddyano/codesearch/cmd/cindex
    go get -u github.com/waddyano/codesearch/cmd/csearch
    go get -u github.com/waddyano/codesearch/cmd/cgrep

## Original Google codesearch README content

    Code Search is a tool for indexing and then performing
    regular expression searches over large bodies of source code.
    It is a set of command-line programs written in Go.
    Binary downloads are available for those who do not have Go installed.
    See https://github.com/google/codesearch.

    For background and an overview of the commands,
    see http://swtch.com/~rsc/regexp/regexp4.html.

    To install:

	go get github.com/google/codesearch/cmd/...

    Use "go get -u" to update an existing installation.

    Russ Cox
    rsc@swtch.com
    June 2015
