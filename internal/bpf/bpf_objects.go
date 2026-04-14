package bpf

import _ "embed"

// PbmonELF is the perf-event-array variant of the compiled eBPF ELF object.
// Used on kernels that do not support BPF_MAP_TYPE_RINGBUF (< 5.8).
// To regenerate: run `make ebpf` which compiles internal/bpf/c/pbmon.c
// using clang into internal/bpf/pbmon_bpf.o.
//
//go:embed pbmon_bpf.o
var PbmonELF []byte

// PbmonELFRingBuf is the ring-buffer variant of the compiled eBPF ELF object.
// Used on kernels that support BPF_MAP_TYPE_RINGBUF (>= 5.8, preferred).
// To regenerate: run `make ebpf` which also compiles internal/bpf/pbmon_bpf_ringbuf.o.
//
//go:embed pbmon_bpf_ringbuf.o
var PbmonELFRingBuf []byte
