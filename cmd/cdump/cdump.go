// Copyright 2011 The Go Authors.  All rights reserved.
// Copyright 2013-2016 Manpreet Singh ( junkblocker@yahoo.com ). All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/waddyano/codesearch/index"
)

var usageMessage = `usage: cdump [options]

Options:

  -c           print only a count of selected lines to stdout
               (Not meaningful with -l or -M modes)
  -f PATHREGEXP
               search only files with names matching this regexp
  -h           print this help text and exit
  -i           case-insensitive search
  -l           print only the names of the files containing matches
               (Not meaningful with -c or -M modes)
  -0           print -l matches separated by NUL ('\0') character
  -m MAXCOUNT  limit search output results to MAXCOUNT (0: no limit)
  -M MAXCOUNT  limit search output results to MAXCOUNT per file (0: no limit)
               (Not allowed with -c or -l modes)
  -n           print each output line preceded by its relative line number in
               the file, starting at 1
  -indexpath FILE
               use specified FILE as the index path. Overrides $CSEARCHINDEX.
  -verbose     print extra information
`

func usage() {
	fmt.Fprintf(os.Stderr, usageMessage)
	os.Exit(2)
}

var (
	verboseFlag = flag.Bool("verbose", false, "print extra information")
	indexPath   = flag.String("indexpath", "", "specifies index path")
)

func Main() {
	flag.Usage = usage
	flag.Parse()
	args := flag.Args()

	if len(args) != 0 {
		usage()
	}

	if *indexPath != "" {
		err := os.Setenv("CSEARCHINDEX", *indexPath)
		if err != nil {
			log.Fatal(err)
		}
	}

	ix := index.Open(index.File())
	ix.Dump()
	ix.Verbose = *verboseFlag
}

func main() {
	Main()
	os.Exit(0)
}
