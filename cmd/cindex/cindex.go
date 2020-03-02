// Copyright 2011 The Go Authors.  All rights reserved.
// Copyright 2013-2014 Manpreet Singh ( junkblocker@yahoo.com ). All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"runtime/pprof"
	"sort"
	"strings"

	"github.com/waddyano/codesearch/index"
)

const (
	DEFAULT_MAX_FILE_LENGTH             = 1 << 30
	DEFAULT_MAX_LINE_LENGTH             = 2000
	DEFAULT_MAX_TEXT_TRIGRAMS           = 30000
	DEFAULT_MAX_INVALID_UTF8_PERCENTAGE = 0.1
)

var usageMessage = `usage: cindex [options] [path...]

Options:

  -verbose     print extra information
  -list        list indexed paths and exit
  -reset       discard existing index
  -indexpath FILE
               use specified FILE as the index path. Overrides $CSEARCHINDEX.
  -cpuprofile FILE
               write CPU profile to FILE
  -logskip     print why a file was skipped from indexing
  -no-follow-symlinks
               do not follow symlinked files and directories
  -maxFileLen BYTES
               skip indexing a file if longer than this size in bytes (Default: %v)
  -maxlinelen BYTES
               skip indexing a file if it has a line longer than this size in bytes (Default: %v)
  -maxtrigrams COUNT
               skip indexing a file if it has more than this number of trigrams (Default: %v)
  -maxinvalidutf8ratio RATIO
               skip indexing a file if it has more than this ratio of invalid UTF-8 sequences (Default: %v)
  -exclude FILE
               path to file containing a list of file patterns to exclude from indexing
  -filelist FILE
               path to file containing a list of file paths to index

cindex prepares the trigram index for use by csearch.  The index is the
file named by $CSEARCHINDEX, or else $HOME/.csearchindex.

The simplest invocation is

	cindex path...

which adds the file or directory tree named by each path to the index.
For example:

	cindex $HOME/src /usr/include

or, equivalently:

	cindex $HOME/src
	cindex /usr/include

If cindex is invoked with no paths, it reindexes the paths that have
already been added, in case the files have changed.  Thus, 'cindex' by
itself is a useful command to run in a nightly cron job.

By default cindex adds the named paths to the index but preserves
information about other paths that might already be indexed
(the ones printed by cindex -list).  The -reset flag causes cindex to
delete the existing index before indexing the new paths.
With no path arguments, cindex -reset removes the index.
`

func usage() {
	fmt.Fprintf(os.Stderr, usageMessage, DEFAULT_MAX_FILE_LENGTH, DEFAULT_MAX_LINE_LENGTH, DEFAULT_MAX_TEXT_TRIGRAMS, DEFAULT_MAX_INVALID_UTF8_PERCENTAGE)
	os.Exit(2)
}

var (
	listFlag             = flag.Bool("list", false, "list indexed paths and exit")
	resetFlag            = flag.Bool("reset", false, "discard existing index")
	verboseFlag          = flag.Bool("verbose", false, "print extra information")
	cpuProfile           = flag.String("cpuprofile", "", "write cpu profile to this file")
	indexPath            = flag.String("indexpath", "", "specifies index path")
	logSkipFlag          = flag.Bool("logskip", false, "print why a file was skipped from indexing")
	noFollowSymlinksFlag = flag.Bool("no-follow-symlinks", false, "do not follow symlinked files and directories")
	exclude              = flag.String("exclude", "", "path to file containing a list of file patterns to exclude from indexing")
	fileList             = flag.String("filelist", "", "path to file containing a list of file paths to index")
	// Tuning variables for detecting text files.
	// A file is assumed not to be text files (and thus not indexed) if
	// 1) if it contains an invalid UTF-8 sequences
	// 2) if it is longer than maxFileLength bytes
	// 3) if it contains a line longer than maxLineLen bytes,
	// or
	// 4) if it contains more than maxTextTrigrams distinct trigrams.
	maxFileLen          = flag.Int64("maxfilelen", DEFAULT_MAX_FILE_LENGTH, "skip indexing a file if longer than this size in bytes")
	maxLineLen          = flag.Int("maxlinelen", DEFAULT_MAX_LINE_LENGTH, "skip indexing a file if it has a line longer than this size in bytes")
	maxTextTrigrams     = flag.Int("maxtrigrams", DEFAULT_MAX_TEXT_TRIGRAMS, "skip indexing a file if it has more than this number of trigrams")
	maxInvalidUTF8Ratio = flag.Float64("maxinvalidutf8ratio", DEFAULT_MAX_INVALID_UTF8_PERCENTAGE, "skip indexing a file if it has more than this ratio of invalid UTF-8 sequences")

	excludePatterns = []string{
		".csearchindex",
	}
)

type walkStats struct {
	nFiles       int
	nDirectories int
	nSkipped     int
	extCounts    map[string]int
}

func walk(rootNo int, arg string, stats *walkStats, symlinkFrom string, out chan struct {
	int
	string
}, logskip bool) {
	filepath.Walk(arg, func(path string, info os.FileInfo, err error) error {
		if info != nil && info.IsDir() {
			stats.nDirectories++
		} else {
			stats.nFiles++
		}
		if stats.nFiles > 0 && stats.nFiles%10000 == 0 {
			log.Printf("scanned %d files, skipped %d", stats.nFiles, stats.nSkipped)
		}
		if basedir, elem := filepath.Split(path); elem != "" {
			exclude := false
			for _, pattern := range excludePatterns {
				exclude, err = filepath.Match(pattern, elem)
				if err != nil {
					log.Fatal(err)
				}
				if exclude {
					break
				}
			}

			// Skip various temporary or "hidden" files or directories.
			if info != nil && info.IsDir() {
				if exclude {
					stats.nSkipped++
					if logskip {
						if symlinkFrom != "" {
							log.Printf("%s: skipped. Excluded directory", symlinkFrom+path[len(arg):])
						} else {
							log.Printf("%s: skipped. Excluded directory", path)
						}
					}
					return filepath.SkipDir
				}
			} else {
				ext := filepath.Ext(path)
				stats.extCounts[ext]++
				if exclude {
					stats.nSkipped++
					if logskip {
						if symlinkFrom != "" {
							log.Printf("%s: skipped. Excluded file", symlinkFrom+path[len(arg):])
						} else {
							log.Printf("%s: skipped. Excluded file", path)
						}
					}
					return nil
				}
				if info != nil && info.Mode()&os.ModeSymlink != 0 {
					if *noFollowSymlinksFlag {
						if logskip {
							log.Printf("%s: skipped. Symlink", path)
						}
						return nil
					}
					var symlinkAs string
					if basedir[len(basedir)-1] == os.PathSeparator {
						symlinkAs = basedir + elem
					} else {
						symlinkAs = basedir + string(os.PathSeparator) + elem
					}
					if symlinkFrom != "" {
						symlinkAs = symlinkFrom + symlinkAs[len(arg):]
					}
					if p, err := filepath.EvalSymlinks(symlinkAs); err != nil {
						if symlinkFrom != "" {
							log.Printf("%s: skipped. Symlink could not be resolved", symlinkFrom+path[len(arg):])
						} else {
							log.Printf("%s: skipped. Symlink could not be resolved", path)
						}
					} else {
						walk(rootNo, p, stats, symlinkAs, out, logskip)
					}
					return nil
				}
			}
		}
		if err != nil {
			if symlinkFrom != "" {
				log.Printf("%s: skipped. Error: %s", symlinkFrom+path[len(arg):], err)
			} else {
				log.Printf("%s: skipped. Error: %s", path, err)
			}
			return nil
		}
		if info != nil {
			if info.Mode()&os.ModeType == 0 {
				if symlinkFrom == "" {
					out <- struct {
						int
						string
					}{rootNo, path}
				} else {
					out <- struct {
						int
						string
					}{-1, symlinkFrom + path[len(arg):]}
				}
			} else if !info.IsDir() {
				if logskip {
					if symlinkFrom != "" {
						log.Printf("%s: skipped. Unsupported path type", symlinkFrom+path[len(arg):])
					} else {
						log.Printf("%s: skipped. Unsupported path type", path)
					}
				}
			}
		} else {
			if logskip {
				if symlinkFrom != "" {
					log.Printf("%s: skipped. Could not stat.", symlinkFrom+path[len(arg):])
				} else {
					log.Printf("%s: skipped. Could not stat.", path)
				}
			}
		}
		return nil
	})
	log.Printf("finished scanning %d directories, %d files, skipped %d", stats.nDirectories, stats.nFiles, stats.nSkipped)
}

func main() {
	flag.Usage = usage
	flag.Parse()
	args := flag.Args()

	if *indexPath != "" {
		if err := os.Setenv("CSEARCHINDEX", *indexPath); err != nil {
			log.Fatal(err)
		}
	}

	if *listFlag {
		master := index.File()
		if stat, err := os.Stat(master); err != nil || stat == nil {
			log.Fatal("Index " + master + " is not accessible")
		} else if stat.IsDir() || !stat.Mode().IsRegular() {
			log.Fatal("Index " + master + " must point to an index file")
		}
		ix := index.Open(master)
		for _, arg := range ix.Paths() {
			fmt.Printf("%s\n", arg)
		}
		return
	}

	if *cpuProfile != "" {
		f, err := os.Create(*cpuProfile)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	if *resetFlag && len(args) == 0 {
		master := index.File()
		stat, err := os.Stat(master)
		if err != nil {
			// does not exist so nothing to do
			return
		}
		if stat != nil && !stat.IsDir() && stat.Mode().IsRegular() {
			os.Remove(master)
			return
		}
		log.Fatal("Invalid index path " + master)
	}

	if *exclude != "" {
		var excludePath string
		if (*exclude)[:2] == "~/" {
			excludePath = filepath.Join(index.HomeDir(), (*exclude)[2:])
		} else {
			excludePath = *exclude
		}
		if *logSkipFlag {
			log.Printf("Loading exclude patterns from %s", excludePath)
		}
		data, err := ioutil.ReadFile(excludePath)
		if err != nil {
			log.Fatal(err)
		}
		excludePatterns = append(excludePatterns, strings.Split(string(data), "\n")...)
		for i, pattern := range excludePatterns {
			excludePatterns[i] = strings.TrimSpace(pattern)
		}
	}

	if *fileList != "" {
		var fileListPath string
		if (*fileList)[:2] == "~/" {
			fileListPath = filepath.Join(index.HomeDir(), (*fileList)[2:])
		} else {
			fileListPath = *fileList
		}
		if *logSkipFlag {
			log.Printf("Loading fileList patterns from %s", fileListPath)
		}
		data, err := ioutil.ReadFile(fileListPath)
		if err != nil {
			log.Fatal(err)
		}
		args = append(args, strings.Split(string(data), "\n")...)
	}

	if len(args) == 0 {
		ix := index.Open(index.File())
		for _, arg := range ix.Paths() {
			args = append(args, arg)
		}
		ix.Close()
	}

	// Translate paths to absolute paths so that we can
	// generate the file list in sorted order.
	for i, arg := range args {
		a, err := filepath.Abs(arg)
		if err != nil {
			log.Printf("%s: %s", arg, err)
			args[i] = ""
			continue
		}
		args[i] = a
	}
	sort.Strings(args)

	for len(args) > 0 && args[0] == "" {
		args = args[1:]
	}

	master := index.File()
	if stat, err := os.Stat(master); err != nil {
		// Does not exist.
		*resetFlag = true
	} else {
		if stat != nil && (stat.IsDir() || !stat.Mode().IsRegular()) {
			log.Fatal("Invalid index path " + master)
		}

	}
	file := master
	if !*resetFlag {
		file += "~"
	}

	ix := index.Create(file)
	ix.Verbose = *verboseFlag
	ix.LogSkip = *logSkipFlag
	ix.MaxFileLen = *maxFileLen
	ix.MaxLineLen = *maxLineLen
	ix.MaxTextTrigrams = *maxTextTrigrams
	ix.MaxInvalidUTF8Ratio = *maxInvalidUTF8Ratio
	ix.AddPaths(args)

	walkChan := make(chan struct {
		int
		string
	}, 10000)
	doneChan := make(chan bool)

	go func() {
		seen := make(map[string]bool)
		nProcessed := 0
		nAdded := 0
		for {
			rootAndPath := <-walkChan
			path := rootAndPath.string
			if path == "" {
				log.Printf("added %d/%d files", nAdded, nProcessed)
				doneChan <- true
				return
			}

			if !seen[path] {
				seen[path] = true
				if ix.AddFile(rootAndPath.int, path) {
					nAdded++
				}
				nProcessed++
				if nProcessed%10000 == 0 {
					log.Printf("added %d/%d files", nAdded, nProcessed)
				}
			}
		}
	}()

	var stats walkStats
	stats.extCounts = make(map[string]int)

	for i, arg := range args {
		log.Printf("index %s", arg)
		walk(i, arg, &stats, "", walkChan, *logSkipFlag)
	}
	log.Printf("walk done %d files %d directories, %d skipped", stats.nFiles, stats.nDirectories, stats.nSkipped)

	extArray := make([]string, 0, len(stats.extCounts))
	for ext, _ := range stats.extCounts {
		extArray = append(extArray, ext)
	}

	sort.Strings(extArray)

	for _, ext := range extArray {
		fmt.Printf("%s %d\n", ext, stats.extCounts[ext])
	}
	//doneChan <- true
	walkChan <- struct {
		int
		string
	}{-1, ""}
	<-doneChan
	log.Printf("flush index")
	ix.Flush()

	if !*resetFlag {
		log.Printf("merge %s %s", master, file)
		index.Merge(file+"~", master, file)
		os.Remove(file)
		os.Remove(master)
		if err := os.Rename(file+"~", master); err != nil {
			log.Fatalf("failed to merge indexes: %s", err)
		}
	}
	log.Printf("done")
	return
}
