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

  -indexpath FILE
			   use specified FILE as the index path. Overrides $CSEARCHINDEX.
  -names       print path names in index
  -verbose     print extra information
`

func usage() {
	fmt.Fprintf(os.Stderr, usageMessage)
	os.Exit(2)
}

var (
	verboseFlag = flag.Bool("verbose", false, "print extra information")
	indexPath   = flag.String("indexpath", "", "specifies index path")
	names       = flag.Bool("names", false, "print path names in index")
)

func Main() {
	flag.Usage = usage
	flag.Parse()
	var options index.DumpOptions
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

	options.Names = *names
	ix := index.Open(index.File())
	ix.Dump(&options)
	ix.Verbose = *verboseFlag
}

func main() {
	Main()
	os.Exit(0)
}
