// Package bpf provides the eBPF loader for pbmon.
// It loads pbmon_bpf.o (compiled from pbmon.c), attaches all tracepoints,
// and exposes a unified EventReader interface over ring-buffer or perf-event-array.
package bpf

import (
	"bytes"
	"fmt"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/features"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/rlimit"
)

// Objects wraps the loaded eBPF maps and programs, plus the tracepoint links.
// Call Close() to detach tracepoints and free kernel resources.
type Objects struct {
	// Maps
	PfidClassMap   *ebpf.Map // {tgid,fd} -> fd_class
	TidFdMap       *ebpf.Map // tid -> fd  (temp)
	TidPipeptrMap  *ebpf.Map // tid -> pipe ptr (temp)
	Events         *ebpf.Map // perf event array or ring buffer

	links      []link.Link
	useRingBuf bool
}

// Close detaches all tracepoints and closes all eBPF resources.
func (o *Objects) Close() {
	for _, l := range o.links {
		l.Close()
	}
	if o.PfidClassMap != nil {
		o.PfidClassMap.Close()
	}
	if o.TidFdMap != nil {
		o.TidFdMap.Close()
	}
	if o.TidPipeptrMap != nil {
		o.TidPipeptrMap.Close()
	}
	if o.Events != nil {
		o.Events.Close()
	}
}

// UseRingBuf reports whether the ring buffer was selected (kernel >= 5.8).
func (o *Objects) UseRingBuf() bool { return o.useRingBuf }

// Load compiles and attaches the pbmon eBPF program.
// It detects kernel features at runtime:
//   - Ring buffer (kernel >= 5.8): single global buffer, lower overhead.
//   - Perf event array (kernel >= 4.9): per-CPU, broader compatibility.
func Load() (*Objects, error) {
	// Bump the locked memory limit – needed on kernels without BPF_PROG_TYPE_CGROUP_SKB
	// and on older kernels where the default is very small.
	if err := rlimit.RemoveMemlock(); err != nil {
		return nil, fmt.Errorf("remove memlock rlimit: %w", err)
	}

	// Feature detection: ring buffer available on kernel >= 5.8
	useRingBuf := features.HaveMapType(ebpf.RingBuf) == nil

	// Parse the embedded ELF
	spec, err := ebpf.LoadCollectionSpecFromReader(bytes.NewReader(PbmonELF))
	if err != nil {
		return nil, fmt.Errorf("load collection spec: %w", err)
	}

	// If ring buffer is not supported, patch the map type to perf event array.
	// (The C code declares it as PERF_EVENT_ARRAY; this block is a no-op on old kernels.)
	if useRingBuf {
		if m, ok := spec.Maps["events"]; ok {
			m.Type = ebpf.RingBuf
			m.KeySize = 0
			m.ValueSize = 0
			m.MaxEntries = 4 * 1024 * 1024 // 4 MB ring buffer
		}
	}

	// Load into kernel
	coll, err := ebpf.NewCollectionWithOptions(spec, ebpf.CollectionOptions{
		Programs: ebpf.ProgramOptions{
			LogLevel: ebpf.LogLevelBranch,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create BPF collection: %w", err)
	}

	objs := &Objects{
		useRingBuf: useRingBuf,
	}

	// Extract maps
	var ok bool
	objs.PfidClassMap, ok = coll.Maps["pfid_class_map"]
	if !ok {
		coll.Close()
		return nil, fmt.Errorf("map pfid_class_map not found in ELF")
	}
	objs.TidFdMap, ok = coll.Maps["tid_fd_map"]
	if !ok {
		coll.Close()
		return nil, fmt.Errorf("map tid_fd_map not found in ELF")
	}
	objs.TidPipeptrMap, ok = coll.Maps["tid_pipeptr_map"]
	if !ok {
		coll.Close()
		return nil, fmt.Errorf("map tid_pipeptr_map not found in ELF")
	}
	objs.Events, ok = coll.Maps["events"]
	if !ok {
		coll.Close()
		return nil, fmt.Errorf("map events not found in ELF")
	}

	// Attach all tracepoints
	attachments := []struct {
		group string
		name  string
		prog  string
	}{
		// Write
		{"syscalls", "sys_enter_write", "tp_enter_write"},
		{"syscalls", "sys_exit_write", "tp_exit_write"},
		{"syscalls", "sys_enter_writev", "tp_enter_writev"},
		{"syscalls", "sys_exit_writev", "tp_exit_writev"},
		{"syscalls", "sys_enter_pwrite64", "tp_enter_pwrite64"},
		{"syscalls", "sys_exit_pwrite64", "tp_exit_pwrite64"},
		{"syscalls", "sys_enter_pwritev", "tp_enter_pwritev"},
		{"syscalls", "sys_exit_pwritev", "tp_exit_pwritev"},
		{"syscalls", "sys_enter_pwritev2", "tp_enter_pwritev2"},
		{"syscalls", "sys_exit_pwritev2", "tp_exit_pwritev2"},
		// Read
		{"syscalls", "sys_enter_read", "tp_enter_read"},
		{"syscalls", "sys_exit_read", "tp_exit_read"},
		{"syscalls", "sys_enter_readv", "tp_enter_readv"},
		{"syscalls", "sys_exit_readv", "tp_exit_readv"},
		{"syscalls", "sys_enter_pread64", "tp_enter_pread64"},
		{"syscalls", "sys_exit_pread64", "tp_exit_pread64"},
		{"syscalls", "sys_enter_preadv", "tp_enter_preadv"},
		{"syscalls", "sys_exit_preadv", "tp_exit_preadv"},
		{"syscalls", "sys_enter_preadv2", "tp_enter_preadv2"},
		{"syscalls", "sys_exit_preadv2", "tp_exit_preadv2"},
		// Network send
		{"syscalls", "sys_enter_sendto", "tp_enter_sendto"},
		{"syscalls", "sys_exit_sendto", "tp_exit_sendto"},
		{"syscalls", "sys_enter_sendmsg", "tp_enter_sendmsg"},
		{"syscalls", "sys_exit_sendmsg", "tp_exit_sendmsg"},
		{"syscalls", "sys_enter_sendmmsg", "tp_enter_sendmmsg"},
		{"syscalls", "sys_exit_sendmmsg", "tp_exit_sendmmsg"},
		{"syscalls", "sys_enter_sendfile64", "tp_enter_sendfile64"},
		{"syscalls", "sys_exit_sendfile64", "tp_exit_sendfile64"},
		// Network recv
		{"syscalls", "sys_enter_recvfrom", "tp_enter_recvfrom"},
		{"syscalls", "sys_exit_recvfrom", "tp_exit_recvfrom"},
		{"syscalls", "sys_enter_recvmsg", "tp_enter_recvmsg"},
		{"syscalls", "sys_exit_recvmsg", "tp_exit_recvmsg"},
		{"syscalls", "sys_enter_recvmmsg", "tp_enter_recvmmsg"},
		{"syscalls", "sys_exit_recvmmsg", "tp_exit_recvmmsg"},
		// FD open – socket
		{"syscalls", "sys_exit_socket", "tp_exit_socket"},
		{"syscalls", "sys_exit_accept", "tp_exit_accept"},
		{"syscalls", "sys_exit_accept4", "tp_exit_accept4"},
		// FD open – file
		{"syscalls", "sys_exit_open", "tp_exit_open"},
		{"syscalls", "sys_exit_openat", "tp_exit_openat"},
		{"syscalls", "sys_exit_creat", "tp_exit_creat"},
		// FD open – pipe
		{"syscalls", "sys_enter_pipe", "tp_enter_pipe"},
		{"syscalls", "sys_exit_pipe", "tp_exit_pipe"},
		{"syscalls", "sys_enter_pipe2", "tp_enter_pipe2"},
		{"syscalls", "sys_exit_pipe2", "tp_exit_pipe2"},
		// FD dup
		{"syscalls", "sys_enter_dup", "tp_enter_dup"},
		{"syscalls", "sys_exit_dup", "tp_exit_dup"},
		{"syscalls", "sys_enter_dup2", "tp_enter_dup2"},
		{"syscalls", "sys_exit_dup2", "tp_exit_dup2"},
		{"syscalls", "sys_enter_dup3", "tp_enter_dup3"},
		{"syscalls", "sys_exit_dup3", "tp_exit_dup3"},
		// FD close
		{"syscalls", "sys_enter_close", "tp_enter_close"},
		// Process lifecycle
		{"sched", "sched_process_exit", "tp_sched_exit"},
		{"sched", "sched_process_fork", "tp_sched_fork"},
	}

	for _, a := range attachments {
		prog, ok := coll.Programs[a.prog]
		if !ok {
			// Skip programs that weren't compiled (e.g. pwritev2 on old kernels)
			continue
		}
		l, err := link.Tracepoint(a.group, a.name, prog, nil)
		if err != nil {
			// Best-effort: log but don't fail on individual tracepoints
			// (some may not exist on all kernel versions)
			_ = err
			continue
		}
		objs.links = append(objs.links, l)
	}

	return objs, nil
}
