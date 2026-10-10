DESTDIR :=
PREFIX := /usr/local

# The version shown by tt -v, taken from the latest tag.
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null)
GOBUILD := CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)"

.PHONY: all
all:
	$(GOBUILD) -o bin/tt ./cmd/tt

.PHONY: install
install:
	install -d $(DESTDIR)$(PREFIX)/bin
	install -d $(DESTDIR)$(PREFIX)/share/man/man1
	install -m755 bin/tt $(DESTDIR)$(PREFIX)/bin
	install -m644 tt.1.gz $(DESTDIR)$(PREFIX)/share/man/man1

.PHONY: uninstall
uninstall:
	rm -f $(DESTDIR)$(PREFIX)/bin/tt
	rm -f $(DESTDIR)$(PREFIX)/share/man/man1/tt.1.gz

.PHONY: assets
assets:
	python3 ./scripts/themegen.py
	pandoc -s -t man -o - man.md|gzip > tt.1.gz

.PHONY: rel
rel:
	GOOS=darwin GOARCH=amd64 go build -o bin/tt-osx ./cmd/tt
	GOOS=windows GOARCH=amd64 go build -o bin/tt.exe ./cmd/tt
	GOOS=linux GOARCH=amd64 go build -o bin/tt-linux ./cmd/tt
	GOOS=linux GOARCH=arm go build -o bin/tt-linux_arm ./cmd/tt
	GOOS=linux GOARCH=arm64 go build -o bin/tt-linux_arm64 ./cmd/tt
