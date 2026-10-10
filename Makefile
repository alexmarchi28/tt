DESTDIR :=
PREFIX := /usr/local

# The version shown by tt -v, taken from the latest tag.
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null)
GOBUILD := CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)"

# The platforms of the release binaries, as os/arch.
PLATFORMS := linux/amd64 linux/arm64 linux/arm darwin/amd64 darwin/arm64 windows/amd64

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

# Builds the release files in bin/release: a binary for each platform, named
# tt-os-arch, the man page and their checksums.
.PHONY: rel
rel:
	rm -rf bin/release
	mkdir -p bin/release
	for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; ext=; [ $$os = windows ] && ext=.exe; \
		GOOS=$$os GOARCH=$$arch $(GOBUILD) -o bin/release/tt-$$os-$$arch$$ext ./cmd/tt || exit 1; \
	done
	cp tt.1.gz bin/release/
	cd bin/release && sha256sum * > SHA256SUMS
