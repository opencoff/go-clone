// progress.go - progess messages
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
	"sync"
	"sync/atomic"
	"time"

	uil "github.com/gosuri/uilive"
	"github.com/opencoff/go-fio"
	"github.com/opencoff/go-fio/clone"
	"github.com/opencoff/go-fio/cmp"
	"github.com/opencoff/go-utils"
)

const (
	_Spinner1 = "-\\|/."
	_Spinner2 = "__\\|/__"

	_Spinner = _Spinner2

	_Spinlen = int64(len(_Spinner))
)

type ProgressOption func(p *Progress)

func WithStats(want bool) ProgressOption {
	return func(p *Progress) {
		p.stats = want
	}
}

func WithVerbose(verb bool) ProgressOption {
	return func(p *Progress) {
		p.verbose = verb
	}
}

// Progress represents a progress bar and verbose message printer
type Progress struct {
	src  atomic.Int64
	dst  atomic.Int64
	spin atomic.Int64
	cp   atomic.Int64
	rm   atomic.Int64

	tot_newd, tot_newf int
	tot_cp, tot_rm     int

	uiw *uil.Writer

	diff *cmp.Difference

	progbar bool
	stats   bool
	verbose bool

	wg    sync.WaitGroup
	ch    chan any
	start time.Time
}

// Create a new progress bar with the given options
func NewProgressBar(show bool, opts ...ProgressOption) (*Progress, error) {
	p := &Progress{
		ch:    make(chan any, 2),
		start: time.Now().UTC(),
	}

	for _, fp := range opts {
		fp(p)
	}

	if show {
		switch IsTTY(os.Stdout) {
		case true:
			p.progbar = true
			p.verbose = false
		case false:
			// not being on a TTY is same as having verbose progress messages
			p.verbose = true
		}
	}

	p.wg.Add(1)
	go p.flusher()

	return p, nil
}

func (p *Progress) Complete() {
	p.showStats()

	close(p.ch)
	p.wg.Wait()
}

func (p *Progress) showStats() {
	if !p.stats {
		return
	}

	var cp, rm, same int64
	var left, right, adds, dels int64
	var wg sync.WaitGroup

	d := p.diff
	wg.Add(4)
	go func() {
		left = count0(d.LeftFiles)
		wg.Done()
	}()
	go func() {
		right = count0(d.RightFiles)
		wg.Done()
	}()
	go func() {
		adds, dels = count1(d.Diff)
		wg.Done()
	}()
	go func() {
		same, _ = count1(d.CommonFiles)
		wg.Done()
	}()
	wg.Wait()

	now := time.Now().UTC()
	tot := now.Sub(p.start).Truncate(time.Millisecond)

	hh := func(n int64) string {
		return utils.HumanizeSize(uint64(n))
	}

	cp = left + adds
	rm = right + dels
	s := fmt.Sprintf(`%s: +%d, -%d, ~%d, =%d; +%s, -%s, =%s`,
		tot.String(), p.tot_newf+p.tot_newd, p.tot_rm, p.tot_cp, d.CommonFiles.Size()+d.CommonDirs.Size(),
		hh(cp), hh(rm), hh(same))

	if p.progbar {
		p.ch <- pstr(s)
	} else {
		p.ch <- vstr(s)
	}
}

type pstr string
type vstr string

func (p *Progress) flusher() {
	var flush func(s string) = func(s string) {}
	var stop func() = func() {}

	if p.progbar {
		w := uil.New()
		w.RefreshInterval = 5 * time.Millisecond
		w.Start()
		flush = func(s string) {
			fmt.Fprintln(w, s)
			w.Flush()
		}
		stop = func() {
			w.Stop()
		}
	}

	for a := range p.ch {
		switch s := a.(type) {
		case pstr:
			flush(string(s))

		case vstr:
			fmt.Println(s)
		}
	}

	stop()
	p.wg.Done()
}

func (p *Progress) v(s string, v ...any) {
	if p.verbose {
		z := fmt.Sprintf(s, v...)
		p.ch <- vstr(z)
	}
}

func (p *Progress) p(s string, v ...any) {
	if p.progbar {
		n := p.spin.Add(1) % _Spinlen
		c := _Spinner[n]
		s := fmt.Sprintf("%s %c", s, c)
		z := fmt.Sprintf(s, v...)
		p.ch <- pstr(z)
	}
}

var _ clone.Observer = &Progress{}
var _ cmp.Observer = &Progress{}

func (p *Progress) VisitSrc(_ *fio.Info) {
	n := p.src.Add(1)
	m := p.dst.Load()
	p.p("clone: scanning src %d, dst %d ..", n, m)
}

func (p *Progress) VisitDst(_ *fio.Info) {
	n := p.src.Load()
	m := p.dst.Add(1)
	p.p("clone: scanning src %d, dst %d ..", n, m)
}

func (p *Progress) Difference(d *cmp.Difference) {
	p.diff = d
	p.tot_cp = d.LeftDirs.Size() + d.LeftFiles.Size() + d.Diff.Size()
	p.tot_rm = d.RightDirs.Size() + d.RightFiles.Size()

	p.p("clone: cp 0/%d, rm 0/%d", p.tot_cp, p.tot_rm)
}

func (p *Progress) Mkdir(dst string) {
	cp := p.cp.Add(1)
	rm := p.rm.Load()

	p.p("clone: cp %d/%d, rm %d/%d", cp, p.tot_cp, rm, p.tot_rm)
	p.v("# mkdir -p %q", dst)
}

func (p *Progress) Copy(dst, src string) {
	cp := p.cp.Add(1)
	rm := p.rm.Load()

	p.p("clone: cp %d/%d, rm %d/%d", cp, p.tot_cp, rm, p.tot_rm)
	p.v("# mkdir -p %q", dst)
}

func (p *Progress) Link(dst, src string) {
	// just show the spinner
	cp := p.cp.Load()
	rm := p.rm.Load()

	p.p("clone: cp %d/%d, rm %d/%d", cp, p.tot_cp, rm, p.tot_rm)
	p.v("# ln  %q %q", src, dst)
}

func (p *Progress) Delete(dst string) {
	cp := p.cp.Load()
	rm := p.rm.Add(1)

	p.p("clone: cp %d/%d, rm %d/%d", cp, p.tot_cp, rm, p.tot_rm)
	p.v("# rm -f %q", dst)
}

func (p *Progress) MetadataUpdate(dst, src string) {
	cp := p.cp.Load()
	rm := p.rm.Load()

	p.p("clone: cp %d/%d, rm %d/%d", cp, p.tot_cp, rm, p.tot_rm)
	p.v("# touch -f %q %q", src, dst)
}

func count0(m *fio.Map) int64 {
	var sz int64

	if m == nil {
		return 0
	}

	m.Range(func(_ string, fi *fio.Info) bool {
		if fi.IsRegular() {
			sz += fi.Size()
		}
		return true
	})
	return sz
}

// count diff bytes
func count1(m *fio.PairMap) (add int64, del int64) {
	if m == nil {
		return 0, 0
	}

	m.Range(func(_ string, p fio.Pair) bool {
		if p.Src.IsRegular() {
			add += p.Src.Size()
			del += p.Dst.Size()
		}
		return true
	})
	return add, del
}

func IsTTY(fd *os.File) bool {
	st, err := fd.Stat()
	if err != nil {
		return false
	}

	return st.Mode()&os.ModeCharDevice > 0
}
