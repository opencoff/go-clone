// dirclone.go -- clone directories

package lib

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"runtime"
	"sync"

	"github.com/opencoff/go-fio"
	"github.com/opencoff/go-fio/cmp"
	"github.com/opencoff/go-fio/walk"
)

type cloneOpt struct {
	ncpu int

	singleFS bool

	followLinks bool

	excludes []string

	filter func(fi *fio.Info) bool
}

func defaultOpts() cloneOpt {
	return cloneOpt{
		ncpu:        runtime.NumCPU(),
		singleFS:    false,
		followLinks: false,
		excludes:    []string{".zfs"},
	}
}

type CloneOption func(o *cloneOpt)

func WithConcurrency(ncpu int) CloneOption {
	return func(o *cloneOpt) {
		if ncpu <= 0 {
			ncpu = runtime.NumCPU()
		}
		o.ncpu = ncpu
	}
}

func WithSingleFS(oneFS bool) CloneOption {
	return func(o *cloneOpt) {
		o.singleFS = oneFS
	}
}

func WithFollowLinks(follow bool) CloneOption {
	return func(o *cloneOpt) {
		o.followLinks = follow
	}
}

func WithExcludes(excl []string) CloneOption {
	return func(o *cloneOpt) {
		o.excludes = append(o.excludes, excl...)
	}
}

func WithFilterFunc(fp func(*fio.Info) bool) CloneOption {
	return func(o *cloneOpt) {
		o.filter = fp
	}
}

func DiffTree(w io.Writer, src, dst string, opts ...CloneOption) error {
	opt := defaultOpts()
	for _, fp := range opts {
		fp(&opt)
	}

	tc, err := newTreeCloner(src, dst, &opt)
	if err != nil {
		return fmt.Errorf("difftree: %w", err)
	}

	tc.workfp = tc.printer
	tc.out = w
	return tc.sync()
}

func CloneTree(src, dst string, opts ...CloneOption) error {
	opt := defaultOpts()
	for _, fp := range opts {
		fp(&opt)
	}

	tc, err := newTreeCloner(src, dst, &opt)
	if err != nil {
		return fmt.Errorf("difftree: %w", err)
	}

	tc.workfp = tc.apply
	return tc.sync()
}

type treeCloner struct {
	cloneOpt

	src, dst string

	diff *cmp.Difference

	workch chan op
	ech    chan error

	workfp func(o op) error
	out    io.Writer
}

// operation to apply and its data
// for opCp: a, b are both valid
// for opRm: only a is valid
type op struct {
	typ  opType
	a, b string

	lhs, rhs *fio.Info
}

type opType int

const (
	opCp opType = 1 + iota // copy a to b
	opRm                   // rm a
)

// clone src to dst; we know both are dirs
func newTreeCloner(src, dst string, opt *cloneOpt) (*treeCloner, error) {
	if err := validate(src, dst); err != nil {
		return nil, err
	}

	wo := walk.Options{
		Concurrency:    opt.ncpu,
		Type:           walk.ALL,
		FollowSymlinks: opt.followLinks,
		OneFS:          opt.singleFS,
		Excludes:       opt.excludes,
		Filter:         opt.filter,
	}

	ltree, err := cmp.NewTree(src, cmp.WithWalkOptions(wo))
	if err != nil {
		return nil, err
	}

	rtree, err := cmp.NewTree(dst, cmp.WithWalkOptions(wo))
	if err != nil {
		return nil, err
	}

	diff, err := cmp.DirCmp(ltree, rtree, cmp.WithIgnore(cmp.IGN_HARDLINK))
	if err != nil {
		return nil, err
	}

	tc := &treeCloner{
		cloneOpt: *opt,
		src:      src,
		dst:      dst,
		diff:     diff,
		workch:   make(chan op, opt.ncpu),
		ech:      make(chan error, 1),
	}

	return tc, nil
}

func validate(src, dst string) error {
	// first make the dir
	di, err := fio.Lstat(dst)
	if err == nil {
		if !di.IsDir() {
			return fmt.Errorf("destination %s is not a directory?", dst)
		}
	}

	if err != nil {
		if os.IsNotExist(err) {
			// make the directory if needed
			err = fio.CloneFile(dst, src)
		}

		if err != nil {
			return err
		}
	}
	return nil
}

func (tc *treeCloner) apply(o op) error {
	var err error

	switch o.typ {
	case opCp:
		err = fio.CloneFile(o.b, o.a)
	case opRm:
		err = os.RemoveAll(o.a)
	default:
	}
	return err
}

func (tc *treeCloner) printer(o op) error {
	switch o.typ {
	case opCp:
		fmt.Fprintf(tc.out, "cp -p '%s' '%s'\n", o.a, o.b)
	case opRm:
		if o.lhs.IsDir() {
			fmt.Fprintf(tc.out, "rm -rf '%s'\n", o.a)
		} else {
			fmt.Fprintf(tc.out, "rm -f '%s'\n", o.a)
		}
	default:
	}
	return nil
}

func (tc *treeCloner) worker(wg *sync.WaitGroup) {
	for o := range tc.workch {
		err := tc.workfp(o)
		if err != nil {
			tc.ech <- err
		}
	}
	wg.Done()
}

func (tc *treeCloner) sync() error {
	var wg sync.WaitGroup
	var errs []error
	var ewg sync.WaitGroup

	// harvest errors
	ewg.Add(1)
	go func() {
		for e := range tc.ech {
			errs = append(errs, e)
		}
		ewg.Done()
	}()

	// start workers
	wg.Add(tc.ncpu)
	for i := 0; i < tc.ncpu; i++ {
		go tc.worker(&wg)
	}

	// And, queue up work for the workers
	var submitDone sync.WaitGroup

	diff := tc.diff
	submitDone.Add(3)
	go func(wg *sync.WaitGroup) {
		for _, nm := range diff.Diff {
			s, ok := diff.Left[nm]
			if !ok {
				tc.ech <- fmt.Errorf("%s: can't find in left map", nm)
				continue
			}

			d, ok := diff.Right[nm]
			if !ok {
				tc.ech <- fmt.Errorf("%s: can't find in right map", nm)
				continue
			}

			o := op{
				typ: opCp,
				a:   s.Name(),
				b:   d.Name(),
				lhs: s,
				rhs: d,
			}
			tc.workch <- o
		}
		wg.Done()
	}(&submitDone)

	go func(wg *sync.WaitGroup) {
		for _, nm := range diff.LeftOnly {
			s, ok := diff.Left[nm]
			if !ok {
				tc.ech <- fmt.Errorf("%s: can't find in left map", nm)
				continue
			}

			o := op{
				typ: opCp,
				a:   s.Name(),
				b:   path.Join(tc.dst, nm),
				lhs: s,
			}
			tc.workch <- o
		}
		wg.Done()
	}(&submitDone)

	go func(wg *sync.WaitGroup) {
		for _, nm := range diff.RightOnly {
			d, ok := diff.Right[nm]
			if !ok {
				tc.ech <- fmt.Errorf("diff: can't find %s in destination", nm)
				continue
			}

			o := op{
				typ: opRm,
				a:   d.Name(),
				lhs: d,
			}
			tc.workch <- o
		}
		wg.Done()
	}(&submitDone)

	// when we're done submitting all the work, close the worker input chan
	go func() {
		submitDone.Wait()
		close(tc.workch)
	}()

	// now wait for workers to complete
	wg.Wait()

	// wait for error harvestor to be complete
	close(tc.ech)
	ewg.Wait()

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}
