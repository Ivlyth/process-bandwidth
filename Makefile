.PHONY: all generate build test clean ebpf env help

# ──────────────────────────────────────────────────────────────
# Tools
# ──────────────────────────────────────────────────────────────

# On macOS, Apple's clang does not include the BPF backend.
# Auto-detect Homebrew LLVM clang (brew install llvm) when on Darwin.
ifeq ($(shell uname -s),Darwin)
  _BREW_PREFIX := $(shell brew --prefix llvm 2>/dev/null)
  ifneq ($(_BREW_PREFIX),)
    CLANG ?= $(_BREW_PREFIX)/bin/clang
  else
    $(error On macOS, Apple clang has no BPF backend. Install upstream LLVM: brew install llvm)
  endif
else
  CLANG ?= clang
endif

GO      ?= go
ARCH    ?= $(shell uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')
UNAME_R ?= $(shell uname -r)

# Git version info
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
VERSION := $(shell git describe --tags --abbrev=0 2>/dev/null || echo "dev")

LDFLAGS := -s -w \
	-X github.com/Ivlyth/process-bandwidth/version.VERSION=$(VERSION) \
	-X github.com/Ivlyth/process-bandwidth/version.COMMIT=$(COMMIT)

# ──────────────────────────────────────────────────────────────
# Default target
# ──────────────────────────────────────────────────────────────

all: ebpf build

# ──────────────────────────────────────────────────────────────
# eBPF compilation (clang → BPF ELF object, embedded in binary)
# ──────────────────────────────────────────────────────────────

BPF_SRC      := internal/bpf/c/pbmon.c
BPF_OBJ      := internal/bpf/pbmon_bpf.o
BPF_OBJ_RB   := internal/bpf/pbmon_bpf_ringbuf.o
BPF_HDRS     := internal/bpf/c/headers

CLANG_FLAGS := \
	-O2 -g \
	-target bpf \
	-D__BPF_TRACING__ \
	-D__KERNEL__ \
	-Wall \
	-Wno-unused-variable \
	-Wno-frame-address \
	-Wno-unused-value \
	-Wno-unknown-warning-option \
	-Wno-pragma-once-outside-header \
	-Wno-pointer-sign \
	-Wno-gnu-variable-sized-type-not-at-end \
	-Wno-deprecated-declarations \
	-Wno-compare-distinct-pointer-types \
	-Wno-address-of-packed-member \
	-fno-stack-protector \
	-fno-jump-tables \
	-fno-unwind-tables \
	-fno-asynchronous-unwind-tables

ebpf: $(BPF_OBJ) $(BPF_OBJ_RB)

# Arch-specific system include path for asm/types.h etc.
# Only added when the directory exists – bpf_helpers.h is self-contained so
# no system Linux headers are required (enables cross-compilation from macOS).
SYSROOT_INC ?= /usr/include/$(shell uname -m | sed 's/x86_64/x86_64-linux-gnu/;s/aarch64/aarch64-linux-gnu/')
SYSROOT_FLAGS := $(shell test -d "$(SYSROOT_INC)" && echo "-I$(SYSROOT_INC)")

$(BPF_OBJ): $(BPF_SRC) $(wildcard $(BPF_HDRS)/*.h)
	$(CLANG) $(CLANG_FLAGS) -I$(BPF_HDRS) $(SYSROOT_FLAGS) -c $< -o $@
	@echo "  eBPF compiled (perf): $@"

$(BPF_OBJ_RB): $(BPF_SRC) $(wildcard $(BPF_HDRS)/*.h)
	$(CLANG) $(CLANG_FLAGS) -DUSE_RINGBUF=1 -I$(BPF_HDRS) $(SYSROOT_FLAGS) -c $< -o $@
	@echo "  eBPF compiled (ringbuf): $@"

# ──────────────────────────────────────────────────────────────
# Go code generation
# (Requires bpf2go: go install github.com/cilium/ebpf/cmd/bpf2go@latest)
# This wraps the compiled ELF in Go source so it can be embedded at build time.
# The generated files (pbmon_bpfel*.go) should be committed to the repo.
# ──────────────────────────────────────────────────────────────

generate: $(BPF_OBJ)
	$(GO) generate ./internal/bpf/...

# ──────────────────────────────────────────────────────────────
# Go build
# ──────────────────────────────────────────────────────────────

build:
	CGO_ENABLED=0 GOOS=linux GOARCH=$(ARCH) \
	  $(GO) build -ldflags "$(LDFLAGS)" -o pbmon ./cmd/pbmon/
	@echo "  Binary: ./pbmon"

# ──────────────────────────────────────────────────────────────
# Tests
# ──────────────────────────────────────────────────────────────

test:
	$(GO) test -race ./...

# ──────────────────────────────────────────────────────────────
# Clean
# ──────────────────────────────────────────────────────────────

clean:
	rm -f pbmon $(BPF_OBJ) $(BPF_OBJ_RB)
	rm -f internal/bpf/pbmon_bpf*_*.go internal/bpf/pbmon_bpf*.o

# ──────────────────────────────────────────────────────────────
# Environment info
# ──────────────────────────────────────────────────────────────

env:
	@echo "ARCH    = $(ARCH)"
	@echo "CLANG   = $(shell $(CLANG) --version 2>/dev/null | head -1)"
	@echo "GO      = $(shell $(GO) version 2>/dev/null)"
	@echo "COMMIT  = $(COMMIT)"
	@echo "VERSION = $(VERSION)"
	@echo "UNAME_R = $(UNAME_R)"

# ──────────────────────────────────────────────────────────────
# Help
# ──────────────────────────────────────────────────────────────

help:
	@echo "Targets:"
	@echo "  all        Build eBPF object and Go binary (default)"
	@echo "  ebpf       Compile eBPF C → BPF ELF object"
	@echo "  generate   Run bpf2go to generate Go bindings from ELF"
	@echo "  build      Compile the Go binary"
	@echo "  test       Run all tests with race detector"
	@echo "  clean      Remove build artifacts"
	@echo "  env        Show build environment"
