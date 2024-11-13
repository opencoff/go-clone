// progress.go -- progress bar for clone

package main

import (
	"fmt"
	"io"
	"sync"
	"sync/atomic"

	"github.com/opencoff/go-fio"
	"github.com/opencoff/go-fio/clone"
	"github.com/opencoff/go-fio/cmp"
	"github.com/opencoff/go-utils"

	"github.com/chelnak/ysmrr"
)

type progress struct {
	pb        ysmrr.SpinnerManager
	showStats bool

	verbose func(s string, a ...interface{}) (int, error)

	line0 *ysmrr.Spinner
	line1 *ysmrr.Spinner

	newfiles int // files + dirs
	same     int
	changed  int

	// total bytes
	newsz   uint64
	delsz   uint64
	chgsz   uint64
	unchgsz uint64

	cp atomic.Int64
	rm atomic.Int64

	nsrc atomic.Int64
	ndst atomic.Int64
}

var _ clone.Observer = &progress{}
var _ cmp.Observer = &progress{}

type progressOption func(p *progress)

func withStats(want bool) progressOption {
	return func(p *progress) {
		p.showStats = want
	}
}

func withVerbose(verb bool) progressOption {
	return func(p *progress) {
		if verb {
			p.verbose = fmt.Printf
		}
	}
}

func progressBar(showProgress bool, opts ...progressOption) (*progress, error) {
	p := &progress{
		verbose: func(s string, a ...any) (int, error) { return len(s), nil },
	}

	if showProgress {
		p.pb = ysmrr.NewSpinnerManager()
		p.line0 = p.pb.AddSpinner(fmt.Sprintf("Scanning src .."))
		p.line1 = p.pb.AddSpinner(fmt.Sprintf("Scanning dst .."))

		go p.pb.Start()
	}

	for _, fp := range opts {
		fp(p)
	}

	return p, nil
}

func (p *progress) VisitSrc(_ *fio.Info) {
	n := p.nsrc.Add(1)
	s := fmt.Sprintf("Scanning src .. %d", n)
	p.line0.UpdateMessage(s)
}

func (p *progress) VisitDst(_ *fio.Info) {
	n := p.ndst.Add(1)
	s := fmt.Sprintf("Scanning dst .. %d", n)
	p.line1.UpdateMessage(s)
}

func (p *progress) Difference(d *cmp.Difference) {
	p.same = d.CommonDirs.Size() + d.CommonFiles.Size()
	p.newfiles = d.LeftDirs.Size() + d.LeftFiles.Size()
	p.changed = d.Diff.Size()

	// calc sizes of changes in bytes
	var wg sync.WaitGroup
	var adds, dels uint64

	wg.Add(4)
	go func() {
		p.newsz = count0(d.LeftFiles)
		wg.Done()
	}()
	go func() {
		p.delsz = count0(d.RightFiles)
		wg.Done()
	}()
	go func() {
		adds, dels = count1(d.Diff)
		wg.Done()
	}()
	go func() {
		p.unchgsz = count2(d.CommonFiles)
		wg.Done()
	}()

	wg.Wait()

	p.delsz += dels
	p.chgsz += adds

	// Now we can add the other bars
	if p.pb != nil {
		p.line0.UpdateMessage("Copying  files ..")
		p.line1.UpdateMessage("Deleting files ..")
	}
}

func (p *progress) complete(wr io.Writer) {
	files := fmt.Sprintf("%d changed, %d deleted, %d added, %d unchanged",
		p.changed, p.rm.Load(), p.newfiles, p.same)
	bytes := fmt.Sprintf("%s copied, %s deleted, %s unchanged",
		utils.HumanizeSize(p.newsz+p.chgsz), utils.HumanizeSize(p.delsz),
		utils.HumanizeSize(p.unchgsz))

	if p.pb != nil {
		p.line0.CompleteWithMessage(files)
		p.line1.CompleteWithMessage(bytes)
		p.pb.Stop()
	} else if p.showStats && wr != nil {
		fmt.Fprintf(wr, "%s\n", files)
		fmt.Fprintf(wr, "%s\n", bytes)
	}
}

func (p *progress) Copy(dst, src string) {
	p.verbose("# cp %q %q\n", src, dst)
	n := p.cp.Add(1)
	if p.pb != nil {
		s := fmt.Sprintf("Copying files .. %d", n)
		p.line0.UpdateMessage(s)
	}
}

func (p *progress) Link(dst, src string) {
	p.verbose("# ln %q %q\n", src, dst)
	n := p.cp.Add(1)
	if p.pb != nil {
		s := fmt.Sprintf("Copying files .. %d", n)
		p.line0.UpdateMessage(s)
	}
}

func (p *progress) Delete(nm string) {
	p.verbose("# rm -f %q\n", nm)
	n := p.cp.Add(1)
	if p.pb != nil {
		s := fmt.Sprintf("Deleting files .. %d", n)
		p.line1.UpdateMessage(s)
	}
}

func (p *progress) MetadataUpdate(dst, src string) {
	p.verbose("# touch --from %q %q\n", src, dst)
}

func count0(m *fio.FioMap) uint64 {
	var sz uint64

	m.Range(func(_ string, fi *fio.Info) bool {
		if fi.IsRegular() {
			sz += uint64(fi.Size())
		}
		return true
	})
	return sz
}

// count diff bytes
func count1(m *fio.FioPairMap) (add uint64, del uint64) {
	m.Range(func(_ string, p fio.Pair) bool {
		if p.Src.IsRegular() {
			add += uint64(p.Src.Size())
			del += uint64(p.Dst.Size())
		}
		return true
	})
	return add, del
}

// count common bytes
func count2(m *fio.FioPairMap) uint64 {
	var sz uint64

	m.Range(func(_ string, p fio.Pair) bool {
		if p.Src.IsRegular() {
			sz += uint64(p.Src.Size())
		}
		return true
	})
	return sz
}
