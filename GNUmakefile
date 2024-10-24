
progs = clone
arch := $(shell ./build --print-arch)
bindir = ./bin/$(arch)

# Always use a command line provided install-dir
ifneq ($(INSTALLDIR),)
    tooldir = $(INSTALLDIR)
else
    tooldir = $(HOME)/bin/$(arch)
endif

.PHONY: clean all $(tooldir) $(progs)

all: $(progs)

install: $(progs) $(tooldir)
	for p in $(progs); do \
		cp $(bindir)/$$p $(tooldir)/ ; \
	done

$(progs):
	./build -s

$(tooldir):
	-mkdir -p $(tooldir)

clean:
	-rm -rf ./bin

