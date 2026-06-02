PREFIX ?= /usr/local
BIN_DIR := $(PREFIX)/bin
SHARE_DIR := $(PREFIX)/share/mping/themes
MAN_DIR := $(PREFIX)/share/man/man1
NAME := mping
SRC := ./cmd/mping
THEMES := $(wildcard themes/*.theme)
MANPAGE := doc/mping.1
GOCACHE ?= /tmp/mping-go-build

.PHONY: all build lint test validate install clean

all: build

build:
	@echo "Building $(NAME)..."
	GOCACHE=$(GOCACHE) go build -o $(NAME) $(SRC)

lint:
	@echo "Checking Go formatting..."
	@test -z "$$(gofmt -l $$(find . -path './macos' -prune -o -name '*.go' -print))"
	@echo "Running go vet..."
	GOCACHE=$(GOCACHE) go vet ./...

test:
	@echo "Running Go tests..."
	GOCACHE=$(GOCACHE) go test ./...

validate: lint test
	@echo "Validating build..."
	GOCACHE=$(GOCACHE) go build ./cmd/mping

install: build
	@echo "Installing $(NAME) to $(BIN_DIR)..."
	mkdir -p $(BIN_DIR)
	install -m 0755 $(NAME) $(BIN_DIR)/$(NAME)
	@echo "Installing themes to $(SHARE_DIR)..."
	mkdir -p $(SHARE_DIR)
	install -m 0644 $(THEMES) $(SHARE_DIR)
	@echo "Installing man page to $(MAN_DIR)..."
	mkdir -p $(MAN_DIR)
	install -m 0644 $(MANPAGE) $(MAN_DIR)/$(NAME).1

clean:
	@echo "Cleaning..."
	rm -f $(NAME)
