//go:build ignore
// +build ignore

// This file is used to trigger bpf2go code generation.
// Run: go generate ./internal/bpf/...
//
// Prerequisites:
//   go install github.com/cilium/ebpf/cmd/bpf2go@latest
//   make ebpf   (compiles pbmon.c → pbmon_bpf.o first)
//
// bpf2go reads pbmon_bpf.o (already compiled by clang via `make ebpf`) and
// generates pbmon_bpfeb.go (big-endian) and pbmon_bpfel.go (little-endian).
// The generated files embed the ELF and expose typed Go access to maps/programs.

package bpf

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go \
//   -cc clang \
//   -no-global-types \
//   -type io_event_t \
//   -type fd_event_t \
//   -type proc_event_t \
//   Pbmon pbmon_bpf.o
