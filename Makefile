ifeq ($(OS),Windows_NT)
    detected_OS := Windows
    EXE_EXT := .exe
    INSTALL_DIR := C:\Program Files\$(BINARY_NAME)
    INSTALL_CMD := copy
    RM := del /Q
else
    detected_OS := $(shell uname -s)
    EXE_EXT :=
    INSTALL_DIR := /usr/local/bin
    INSTALL_CMD := install -m 755
    RM := rm -f
endif

.PHONY: build install clean

BINARY_NAME=defender$(EXE_EXT)

GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean

build:
	CGO_ENABLED=1 $(GOBUILD) -o $(BINARY_NAME)

install:
	$(INSTALL_CMD) $(BINARY_NAME) $(INSTALL_DIR)

clean:
	$(GOCLEAN)
	$(RM) $(BINARY_NAME)