// main.go - main for clone

package main

import (
	"fmt"
	"os"
	"path"
	"runtime"

	"github.com/opencoff/go-clone/lib"
	flag "github.com/opencoff/pflag"
)

var Z = path.Base(os.Args[0])

func main() {
	var help, ver bool
	var progress, apply bool
	var onefs, follow bool
	var ncpu int = runtime.NumCPU()

	fs := flag.NewFlagSet(Z, flag.ExitOnError)

	fs.BoolVarP(&help, "help", "h", false, "Show help and exit [False]")
	fs.BoolVarP(&ver, "version", "", false, "Show version info and exit [False]")
	fs.BoolVarP(&progress, "progress", "p", false, "Show progress bar [False]")
	fs.BoolVarP(&apply, "apply", "", false, "Make the changes [False]")

	fs.IntVarP(&ncpu, "concurrency", "c", ncpu, "Use upto `N` concurrent CPUs [auto-detect]")
	fs.BoolVarP(&follow, "follow-symlinks", "L", false, "Follow symlinks [False]")
	fs.BoolVarP(&onefs, "single-file-system", "x", false, "Don't cross file-system mount points [False]")

	fs.SetOutput(os.Stdout)

	err := fs.Parse(os.Args[1:])
	if err != nil {
		Die("%s", err)
	}

	if help {
		usage(fs)
	}

	if ver {
		fmt.Printf("%s: %s [%s]\n", Z, ProductVersion, RepoVersion)
		os.Exit(0)
	}

	args := fs.Args()
	if len(args) == 0 {
		Die("Usage: %s SRC DEST", Z)
	}

	src := args[0]
	dst := args[1]

	if apply {
		err = lib.CloneTree(src, dst, lib.WithConcurrency(ncpu),
			lib.WithSingleFS(onefs), lib.WithFollowLinks(follow))
	} else {
		err = lib.DiffTree(os.Stdout, src, dst, lib.WithConcurrency(ncpu),
			lib.WithSingleFS(onefs), lib.WithFollowLinks(follow))
	}

	if err != nil {
		Die("%s", err)
	}
}

func usage(fs *flag.FlagSet) {
	fmt.Printf(usageStr, Z, Z)
	fs.PrintDefaults()
	os.Exit(1)
}

var usageStr = `%s - efficiently clone a directory

Usage: %s [options] src dest

%s faithfully duplicates the contents of SRC to DEST/ by
only copying over files that are changed. All file metadata
is duplicated - including xattr when possible.

Options:
`

// will be filled by the build script
var ProductVersion = "UNKNOWN"
var RepoVersion = "UNKNOWN"
