BIN    ?= specsync
PREFIX ?= $(HOME)/.local/bin

# Version stamped into the binary: the nearest tag plus distance and commit
# (0.15.0, or 0.15.0-3-gabc1234, with -dirty for local edits). No --always:
# without a tag the stamp is empty and the binary stays "dev" rather than
# reporting a bare commit id. Override with VERSION=x.y.z.
VERSION ?= $(shell git describe --tags --dirty 2>/dev/null | sed 's/^v//')
LDFLAGS := $(if $(VERSION),-X main.version=$(VERSION))

.PHONY: build test vet install clean sync-skill version

build:
	go build -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/specsync

version:
	@echo $(VERSION)

test:
	go test ./...

vet:
	go vet ./...

install: build
	install -d $(PREFIX)
	install -m 0755 $(BIN) $(PREFIX)/$(BIN)
	@echo "installed $(BIN) -> $(PREFIX)/$(BIN)"

clean:
	rm -f $(BIN)

# Propagate the canonical skill to the two derived locations.
# Run this whenever skills/specsync/SKILL.md changes.
sync-skill:
	cp skills/specsync/SKILL.md cmd/specsync/SKILL.md
	cp skills/specsync/SKILL.md npm/skills/specsync/SKILL.md
	cp skills/specsync/SKILL.md .claude/skills/specsync/SKILL.md
