// main.go - main for clone

package main

import (
	"fmt"
	"os"
	"path"
	"runtime"
	"strings"

	"github.com/opencoff/go-fio"
	"github.com/opencoff/go-fio/clone"
	"github.com/opencoff/go-fio/cmp"
	"github.com/opencoff/go-fio/walk"
	"github.com/puzpuzpuz/xsync/v3"

	flag "github.com/opencoff/pflag"
)

var Z = path.Base(os.Args[0])

func main() {
	var help, ver bool
	var verbose, progress, apply bool
	var onefs, follow, stats, ign bool
	var ncpu int = runtime.NumCPU()

	fs := flag.NewFlagSet(Z, flag.ExitOnError)

	fs.BoolVarP(&help, "help", "h", false, "Show help and exit [False]")
	fs.BoolVarP(&ver, "version", "", false, "Show version info and exit [False]")
	fs.BoolVarP(&progress, "progress", "p", false, "Show progress bar [False]")
	fs.BoolVarP(&apply, "apply", "", false, "Make the changes [False]")

	fs.IntVarP(&ncpu, "concurrency", "c", ncpu, "Use upto `N` concurrent CPUs [auto-detect]")
	fs.BoolVarP(&follow, "follow-symlinks", "L", false, "Follow symlinks [False]")
	fs.BoolVarP(&onefs, "single-file-system", "x", false, "Don't cross file-system mount points [False]")
	fs.BoolVarP(&stats, "show-stats", "s", false, "Show clone statistics in the end [False]")
	fs.BoolVarP(&verbose, "verbose", "v", false, "Show verbose progress messages [False]")
	fs.BoolVarP(&ign, "ignore-missing", "", false, "Ignore files that suddenly disappear [False]")

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

	si, err := fio.Lstat(src)
	if err != nil {
		Die("can't stat %s: %s", src, err)
	}

	if !si.IsDir() {
		Die("%s is not a dir", src)
	}

	di, err := fio.Lstat(dst)
	if err != nil {
		if !os.IsNotExist(err) {
			Die("can't stat %s: %s", dst, err)
		}
	} else if !di.IsDir() {
		Die("%s is not a dir", dst)
	}

	// if both are set, pick verbosity
	if verbose && progress {
		progress = false
	}

	pb, err := progressBar(progress, withStats(stats), withVerbose(verbose))
	if err != nil {
		Die("can't make progress bar: %s", err)
	}

	wo := walk.Options{
		Concurrency:    ncpu,
		Type:           walk.ALL,
		FollowSymlinks: follow,
		OneFS:          onefs,
	}

	if apply {
		err = clone.Tree(dst, src, clone.WithObserver(pb), clone.WithWalkOptions(wo), clone.WithIgnoreMissing(ign))
		if err != nil {
			Die("%s", err)
		}

		pb.complete(os.Stdout)
	} else {
		d, err := cmp.DirTree(src, dst, cmp.WithWalkOptions(wo))
		if err != nil {
			Die("%s", err)
		}
		printDiff(d)
	}
}

func dump[K comparable, V any](pref string, m *xsync.MapOf[K, V]) string {
	var b strings.Builder

	dumpx(&b, pref, m)
	return b.String()
}

func dumpx[K comparable, V any](b *strings.Builder, pref string, m *xsync.MapOf[K, V]) {
	if m.Size() <= 0 {
		return
	}

	if len(pref) > 0 {
		fmt.Fprintf(b, "%s:\n", pref)
	}
	m.Range(func(k K, _ V) bool {
		fmt.Fprintf(b, "\t%s\n", k)
		return true
	})
}

func printDiff(d *cmp.Difference) {
	var b strings.Builder

	dumpx(&b, "New Dirs", d.LeftDirs)
	dumpx(&b, "New Files", d.LeftFiles)

	dumpx(&b, "Modified files", d.Diff)

	dumpx(&b, "Delete dirs", d.RightDirs)
	dumpx(&b, "Delete files", d.RightDirs)

	os.Stdout.WriteString(b.String())
}

func usage(fs *flag.FlagSet) {
	fmt.Printf(usageStr, Z, Z, Z)
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
