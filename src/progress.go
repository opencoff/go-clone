// progress.go -- progress bar for clone

package main

import (
	"github.com/opencoff/go-fio/clone"
	"github.com/opencoff/go-fio/cmp"
)

type progress struct {
}

var _ clone.Observer = &progress{}

func progressBar(want bool) clone.Observer {
	if !want {
		return clone.NopObserver()
	}

	return &progress{}
}

func (d *progress) Difference(_ *cmp.Difference) {
}

func (d *progress) Copy(_, _ string) {
}

func (d *progress) Delete(_ string) {
}

func (d *progress) MetadataUpdate(_, _ string) {
}
