module github.com/opencoff/go-clone

go 1.22.2

require (
	github.com/opencoff/go-fio v0.3.1
	github.com/opencoff/pflag v1.0.6-sh1
)

require (
	github.com/opencoff/go-mmap v0.1.3 // indirect
	github.com/pkg/xattr v0.4.10 // indirect
	golang.org/x/sys v0.26.0 // indirect
)

replace (
	github.com/opencoff/go-fio => ../go-fio
	)
