BIN    ?= specsync
PREFIX ?= $(HOME)/.local/bin

.PHONY: build test vet install clean sync-skill

build:
	go build -o $(BIN) ./cmd/specsync

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
