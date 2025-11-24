PREFIX ?= /usr/local
BIN_DIR := $(PREFIX)/bin
SHARE_DIR := $(PREFIX)/share/mping/themes
MAN_DIR := $(PREFIX)/share/man/man1
NAME := mping
SRC := ./cmd/mping
THEMES := $(wildcard themes/*.theme)
MANPAGE := doc/mping.1

.PHONY: all build install clean

all: build

build:
	@echo "Building $(NAME)..."
	go build -o $(NAME) $(SRC)

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
