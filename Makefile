BIN    ?= specsync
PREFIX ?= $(HOME)/.local/bin

.PHONY: build test vet install clean

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
