// main.go - main for clone
//
// (c) 2024- Sudhi Herle <sudhi@herle.net>
//
// Licensing Terms: GPLv2
//
// If you need a commercial license for this work, please contact
// the author.
//
// This software does not come with any express or implied
// warranty; it is provided "as is". No claim  is made to its
// suitability for any purpose.

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

	flag "github.com/opencoff/pflag"
)

var Z = path.Base(os.Args[0])

func main() {
	var help, ver bool
	var verbose, progress, dryrun bool
	var onefs, follow, stats, ign bool
	var ncpu int = runtime.NumCPU()
	var excl []string

	fs := flag.NewFlagSet(Z, flag.ExitOnError)

	fs.BoolVarP(&help, "help", "h", false, "Show help and exit [False]")
	fs.BoolVarP(&ver, "version", "", false, "Show version info and exit [False]")
	fs.BoolVarP(&progress, "progress", "p", false, "Show progress bar [False]")
	fs.BoolVarP(&dryrun, "dry-run", "n", false, "Show the changes without making them [False]")

	fs.IntVarP(&ncpu, "concurrency", "c", ncpu, "Use upto `N` concurrent CPUs [auto-detect]")
	fs.BoolVarP(&follow, "follow-symlinks", "L", false, "Follow symlinks [False]")
	fs.BoolVarP(&onefs, "single-file-system", "x", false, "Don't cross file-system mount points [False]")
	fs.BoolVarP(&stats, "show-stats", "s", false, "Show clone statistics in the end [False]")
	fs.BoolVarP(&verbose, "verbose", "v", false, "Show verbose progress messages [False]")
	fs.BoolVarP(&ign, "ignore-missing", "", false, "Ignore files that suddenly disappear [False]")
	fs.StringSliceVarP(&excl, "exclude", "X", []string{}, "Excludes files/dirs matching the shell glob pattern")

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
		FollowSymlinks: follow,
		OneFS:          onefs,
		Type:           walk.FILE | walk.DIR | walk.SYMLINK,
		Excludes:       excl,
	}

	if !dryrun {
		err = clone.Tree(dst, src, clone.WithObserver(pb), clone.WithWalkOptions(wo), clone.WithIgnoreMissing(ign))
		if err != nil {
			Die("%s", err)
		}

		pb.complete()
	} else {
		d, err := cmp.FsTree(src, dst, cmp.WithWalkOptions(wo))
		if err != nil {
			Die("%s", err)
		}
		printDiff(d)
	}
}

func printDiff(d *cmp.Difference) {
	var b strings.Builder

	dump0 := func(b *strings.Builder, pref string, m *fio.FioMap) {
		if m.Size() <= 0 {
			return
		}

		m.Range(func(nm string, _ *fio.Info) bool {
			fmt.Fprintf(b, "# %s %s/%s\n", pref, d.Dst, nm)
			return true
		})
	}

	dump1 := func(b *strings.Builder, pref string, m *fio.FioMap) {
		if m.Size() <= 0 {
			return
		}

		m.Range(func(nm string, _ *fio.Info) bool {
			fmt.Fprintf(b, "# %s %s/%s %s/%s\n", pref, d.Src, nm, d.Dst, nm)
			return true
		})
	}

	dump2 := func(b *strings.Builder, pref string, m *fio.FioPairMap) {
		if m.Size() <= 0 {
			return
		}

		m.Range(func(nm string, p fio.Pair) bool {
			fmt.Fprintf(b, "# %s %s %s\n", pref, p.Src.Name(), p.Dst.Name())
			return true
		})
	}

	dump0(&b, "mkdir -p", d.LeftDirs)
	dump0(&b, "rm -rf", d.RightDirs)
	dump0(&b, "rm -f", d.RightFiles)

	dump1(&b, "cp", d.LeftFiles)
	dump2(&b, "cp", d.Diff)

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
