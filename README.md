# go-clone - a concurrent directory cloner

## What is this?
`go-clone` is a tool to clone a directory tree and its contents - preserving metadata (including
xattr).

It is like `cp -pr` except it is much faster (uses golang concurrency everywhere possible).

## How do I build it?
You need a modern golang toolchain (1.22+):

    git clone https://github.com/opencoff/go-clone
    cd go-clone
    make

The resulting binary `clone` will be in `./bin/$GOOS-$GOARCH/`.
eg if you are building this on Linux for x86\_64, the binary will be
`./bin/linux-amd64/clone`.

For now, `clone` is only supported on Linux, OpenBSD, macOS (darwin). Patches
for other platforms welcome.

## Usage with examples
`clone` supports the following options:

      -c, --concurrency N        Use upto N concurrent CPUs [auto-detect]
      -n, --dry-run              Show the changes without making them [False]
      -X, --exclude strings      Excludes files/dirs matching the shell glob pattern
      -L, --follow-symlinks      Follow symlinks [False]
      -h, --help                 Show help and exit [False]
          --ignore-missing       Ignore files that suddenly disappear [False]
      -p, --progress             Show progress bar [False]
      -s, --show-stats           Show clone statistics in the end [False]
      -x, --single-file-system   Don't cross file-system mount points [False]
      -v, --verbose              Show verbose progress messages [False]
          --version              Show version info and exit [False]

By default, clone like other unix tools is quiet; for interactive uses, one
can ask to see some statistics or see progress. eg.,

    clone --exclude .zfs -x -s /tank/volume /backup


## Technical Details
`clone` is largely a command line "wrapper" around a library that exposes
the core functionality [go-fio](https://github.com/opencoff/go-fio).

That library has functionality for:

- concurrently walking a file system tree - like `os.Walk()` but faster
- comparing two directory trees and returing their difference
- cloning a file and its metadata
- cloning a dir tree and associated metadata

The `go-fio` library is tested on macOS/darwin and linux. Patches for other OSes
(especially windows) are welcome. 

