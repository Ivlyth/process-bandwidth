package bpf

import _ "embed"

// PbmonELF is the compiled eBPF ELF object embedded at build time.
// To regenerate: run `make ebpf` which compiles internal/bpf/pbmon.c
// using clang into internal/bpf/pbmon_bpf.o.
//
//go:embed pbmon_bpf.o
var PbmonELF []byte
