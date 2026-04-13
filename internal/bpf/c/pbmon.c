// pbmon.c – process bandwidth monitor eBPF kernel program.
//
// Supports kernel >= 4.9 via syscall tracepoints + perf event array.
// When loaded on kernel >= 5.8 the Go-side loader will swap the events map
// to BPF_MAP_TYPE_RINGBUF before loading (detected at runtime).
//
// Build: go generate ./internal/bpf/...
//   (which runs bpf2go and embeds the compiled ELF into pbmon_bpfel.go)

// clang-format off
#include "vmlinux.h"
#include "bpf_helpers.h"
#include "bpf_tracing.h"
#include "bpf_core_read.h"
// clang-format on

// ──────────────────────────────────────────────────────────────
// Constants
// ──────────────────────────────────────────────────────────────

#define EV_IO   0
#define EV_FD   1
#define EV_PROC 2

#define FD_CLASS_UNKNOWN 0
#define FD_CLASS_SOCKET  1
#define FD_CLASS_FILE    2
#define FD_CLASS_PIPE    3

#define DIR_WRITE 0
#define DIR_READ  1

#define FD_OP_OPEN  0
#define FD_OP_CLOSE 1

#define PROC_OP_EXIT 0
#define PROC_OP_FORK 1

// ──────────────────────────────────────────────────────────────
// Event structs (packed so Go binary.Read works without padding)
// ──────────────────────────────────────────────────────────────

struct io_event_t {
    __u8  ev_type;    // EV_IO
    __u32 pid;        // tgid (process)
    __u32 tid;        // tid  (thread)
    __u32 fd;
    __u8  fd_class;   // FD_CLASS_*
    __u8  direction;  // DIR_WRITE / DIR_READ
    __u64 bytes;
} __attribute__((packed));

struct fd_event_t {
    __u8  ev_type;    // EV_FD
    __u32 pid;
    __u32 tid;
    __u32 fd;
    __u8  fd_class;
    __u8  op;         // FD_OP_OPEN / FD_OP_CLOSE
} __attribute__((packed));

struct proc_event_t {
    __u8  ev_type;    // EV_PROC
    __u32 pid;
    __u32 tid;
    __u8  op;         // PROC_OP_EXIT / PROC_OP_FORK
} __attribute__((packed));

// ──────────────────────────────────────────────────────────────
// Map key: (tgid, fd)  – identifies an open FD within a process.
// We key by tgid (not tid) because the FD table is per-process.
// The tid_fd_map is keyed by tid to pair enter/exit tracepoints.
// ──────────────────────────────────────────────────────────────

struct pfid_key_t {
    __u32 tgid;
    __u32 fd;
};

// ──────────────────────────────────────────────────────────────
// Maps
// ──────────────────────────────────────────────────────────────

// pfid_class_map: (tgid, fd) -> fd_class
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, struct pfid_key_t);
    __type(value, __u8);
    __uint(max_entries, 131072);
} pfid_class_map SEC(".maps");

// tid_fd_map: tid -> fd  (temporary, stores the fd seen at syscall enter
// so the syscall exit handler can look it up)
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, __u32);   // tid
    __type(value, __u32); // fd
    __uint(max_entries, 131072);
} tid_fd_map SEC(".maps");

// tid_pipeptr_map: tid -> userspace pointer to int[2]
// used to pass the pipefd pointer from pipe/pipe2 enter to exit
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, __u32);   // tid
    __type(value, __u64); // ptr
    __uint(max_entries, 4096);
} tid_pipeptr_map SEC(".maps");

// events: output ring (perf event array; Go loader may swap to ringbuf)
struct {
    __uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
    __uint(max_entries, 0); // 0 -> kernel sets to nr_cpus
    __uint(key_size, sizeof(__u32));
    __uint(value_size, sizeof(__u32));
} events SEC(".maps");

// ──────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────

static __always_inline void save_fd_for_tid(__u32 tid, __u32 fd)
{
    bpf_map_update_elem(&tid_fd_map, &tid, &fd, BPF_ANY);
}

static __always_inline __u32 pop_fd_for_tid(__u32 tid)
{
    __u32 *fdp = bpf_map_lookup_elem(&tid_fd_map, &tid);
    if (!fdp) return 0;
    __u32 fd = *fdp;
    bpf_map_delete_elem(&tid_fd_map, &tid);
    return fd;
}

static __always_inline __u8 get_fd_class(__u32 tgid, __u32 fd)
{
    struct pfid_key_t k = {.tgid = tgid, .fd = fd};
    __u8 *cp = bpf_map_lookup_elem(&pfid_class_map, &k);
    return cp ? *cp : FD_CLASS_UNKNOWN;
}

static __always_inline void set_fd_class(__u32 tgid, __u32 fd, __u8 cls)
{
    struct pfid_key_t k = {.tgid = tgid, .fd = fd};
    bpf_map_update_elem(&pfid_class_map, &k, &cls, BPF_ANY);
}

static __always_inline void del_fd_class(__u32 tgid, __u32 fd)
{
    struct pfid_key_t k = {.tgid = tgid, .fd = fd};
    bpf_map_delete_elem(&pfid_class_map, &k);
}

static __always_inline void emit_io_event(void *ctx,
                                          __u32 tgid, __u32 tid,
                                          __u32 fd, __u8 cls,
                                          __u8 dir, __u64 bytes)
{
    struct io_event_t ev = {};
    ev.ev_type   = EV_IO;
    ev.pid       = tgid;
    ev.tid       = tid;
    ev.fd        = fd;
    ev.fd_class  = cls;
    ev.direction = dir;
    ev.bytes     = bytes;
    bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, &ev, sizeof(ev));
}

static __always_inline void emit_fd_event(void *ctx,
                                          __u32 tgid, __u32 tid,
                                          __u32 fd, __u8 cls, __u8 op)
{
    struct fd_event_t ev = {};
    ev.ev_type  = EV_FD;
    ev.pid      = tgid;
    ev.tid      = tid;
    ev.fd       = fd;
    ev.fd_class = cls;
    ev.op       = op;
    bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, &ev, sizeof(ev));
}

static __always_inline void emit_proc_event(void *ctx,
                                            __u32 tgid, __u32 tid, __u8 op)
{
    struct proc_event_t ev = {};
    ev.ev_type = EV_PROC;
    ev.pid     = tgid;
    ev.tid     = tid;
    ev.op      = op;
    bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, &ev, sizeof(ev));
}

// ──────────────────────────────────────────────────────────────
// Generic enter handler: save first argument (fd) for this tid.
// ──────────────────────────────────────────────────────────────

static __always_inline int enter_rw(struct trace_event_raw_sys_enter *ctx)
{
    __u64 ptgid = bpf_get_current_pid_tgid();
    __u32 tid   = (__u32)(ptgid & 0xFFFFFFFF);
    __u32 fd    = (__u32)ctx->args[0];
    save_fd_for_tid(tid, fd);
    return 0;
}

// ──────────────────────────────────────────────────────────────
// Generic exit handler: emit IO event using the saved fd.
// ──────────────────────────────────────────────────────────────

static __always_inline int exit_rw(struct trace_event_raw_sys_exit *ctx,
                                   __u8 direction)
{
    __s64 ret = (__s64)ctx->ret;
    if (ret <= 0) {
        // Still need to consume the saved fd to avoid map leaks.
        __u64 ptgid = bpf_get_current_pid_tgid();
        __u32 tid   = (__u32)(ptgid & 0xFFFFFFFF);
        pop_fd_for_tid(tid);
        return 0;
    }

    __u64 ptgid = bpf_get_current_pid_tgid();
    __u32 tgid  = (__u32)(ptgid >> 32);
    __u32 tid   = (__u32)(ptgid & 0xFFFFFFFF);

    __u32 fd = pop_fd_for_tid(tid);
    if (fd <= 2) return 0; // skip stdin/stdout/stderr

    __u8 cls = get_fd_class(tgid, fd);
    // FD_CLASS_UNKNOWN is emitted too – the Go side may resolve it via /proc.
    emit_io_event(ctx, tgid, tid, fd, cls, direction, (__u64)ret);
    return 0;
}

// ──────────────────────────────────────────────────────────────
// Generic exit handler for fd-creation syscalls (socket/open/etc.).
// ──────────────────────────────────────────────────────────────

static __always_inline int exit_fd_open(struct trace_event_raw_sys_exit *ctx,
                                        __u8 cls)
{
    __s32 ret = (__s32)ctx->ret;
    if (ret < 0) return 0;

    __u64 ptgid = bpf_get_current_pid_tgid();
    __u32 tgid  = (__u32)(ptgid >> 32);
    __u32 tid   = (__u32)(ptgid & 0xFFFFFFFF);
    __u32 fd    = (__u32)ret;

    set_fd_class(tgid, fd, cls);
    emit_fd_event(ctx, tgid, tid, fd, cls, FD_OP_OPEN);
    return 0;
}

// ──────────────────────────────────────────────────────────────
// WRITE syscalls
// ──────────────────────────────────────────────────────────────

SEC("tracepoint/syscalls/sys_enter_write")
int tp_enter_write(struct trace_event_raw_sys_enter *ctx) { return enter_rw(ctx); }
SEC("tracepoint/syscalls/sys_exit_write")
int tp_exit_write(struct trace_event_raw_sys_exit *ctx)   { return exit_rw(ctx, DIR_WRITE); }

SEC("tracepoint/syscalls/sys_enter_writev")
int tp_enter_writev(struct trace_event_raw_sys_enter *ctx) { return enter_rw(ctx); }
SEC("tracepoint/syscalls/sys_exit_writev")
int tp_exit_writev(struct trace_event_raw_sys_exit *ctx)   { return exit_rw(ctx, DIR_WRITE); }

SEC("tracepoint/syscalls/sys_enter_pwrite64")
int tp_enter_pwrite64(struct trace_event_raw_sys_enter *ctx) { return enter_rw(ctx); }
SEC("tracepoint/syscalls/sys_exit_pwrite64")
int tp_exit_pwrite64(struct trace_event_raw_sys_exit *ctx)   { return exit_rw(ctx, DIR_WRITE); } // BUG FIX: was DIR_READ

SEC("tracepoint/syscalls/sys_enter_pwritev")
int tp_enter_pwritev(struct trace_event_raw_sys_enter *ctx) { return enter_rw(ctx); }
SEC("tracepoint/syscalls/sys_exit_pwritev")
int tp_exit_pwritev(struct trace_event_raw_sys_exit *ctx)   { return exit_rw(ctx, DIR_WRITE); } // BUG FIX: was DIR_READ

SEC("tracepoint/syscalls/sys_enter_pwritev2")
int tp_enter_pwritev2(struct trace_event_raw_sys_enter *ctx) { return enter_rw(ctx); }
SEC("tracepoint/syscalls/sys_exit_pwritev2")
int tp_exit_pwritev2(struct trace_event_raw_sys_exit *ctx)   { return exit_rw(ctx, DIR_WRITE); }

// ──────────────────────────────────────────────────────────────
// READ syscalls
// ──────────────────────────────────────────────────────────────

SEC("tracepoint/syscalls/sys_enter_read")
int tp_enter_read(struct trace_event_raw_sys_enter *ctx) { return enter_rw(ctx); }
SEC("tracepoint/syscalls/sys_exit_read")
int tp_exit_read(struct trace_event_raw_sys_exit *ctx)   { return exit_rw(ctx, DIR_READ); }

SEC("tracepoint/syscalls/sys_enter_readv")
int tp_enter_readv(struct trace_event_raw_sys_enter *ctx) { return enter_rw(ctx); }
SEC("tracepoint/syscalls/sys_exit_readv")
int tp_exit_readv(struct trace_event_raw_sys_exit *ctx)   { return exit_rw(ctx, DIR_READ); }

SEC("tracepoint/syscalls/sys_enter_pread64")
int tp_enter_pread64(struct trace_event_raw_sys_enter *ctx) { return enter_rw(ctx); }
SEC("tracepoint/syscalls/sys_exit_pread64")
int tp_exit_pread64(struct trace_event_raw_sys_exit *ctx)   { return exit_rw(ctx, DIR_READ); }

SEC("tracepoint/syscalls/sys_enter_preadv")
int tp_enter_preadv(struct trace_event_raw_sys_enter *ctx) { return enter_rw(ctx); }
SEC("tracepoint/syscalls/sys_exit_preadv")
int tp_exit_preadv(struct trace_event_raw_sys_exit *ctx)   { return exit_rw(ctx, DIR_READ); }

SEC("tracepoint/syscalls/sys_enter_preadv2")
int tp_enter_preadv2(struct trace_event_raw_sys_enter *ctx) { return enter_rw(ctx); }
SEC("tracepoint/syscalls/sys_exit_preadv2")
int tp_exit_preadv2(struct trace_event_raw_sys_exit *ctx)   { return exit_rw(ctx, DIR_READ); }

// ──────────────────────────────────────────────────────────────
// NETWORK SEND syscalls (first arg = sockfd)
// ──────────────────────────────────────────────────────────────

SEC("tracepoint/syscalls/sys_enter_sendto")
int tp_enter_sendto(struct trace_event_raw_sys_enter *ctx) { return enter_rw(ctx); }
SEC("tracepoint/syscalls/sys_exit_sendto")
int tp_exit_sendto(struct trace_event_raw_sys_exit *ctx)   { return exit_rw(ctx, DIR_WRITE); }

SEC("tracepoint/syscalls/sys_enter_sendmsg")
int tp_enter_sendmsg(struct trace_event_raw_sys_enter *ctx) { return enter_rw(ctx); }
SEC("tracepoint/syscalls/sys_exit_sendmsg")
int tp_exit_sendmsg(struct trace_event_raw_sys_exit *ctx)   { return exit_rw(ctx, DIR_WRITE); }

SEC("tracepoint/syscalls/sys_enter_sendmmsg")
int tp_enter_sendmmsg(struct trace_event_raw_sys_enter *ctx) { return enter_rw(ctx); }
SEC("tracepoint/syscalls/sys_exit_sendmmsg")
int tp_exit_sendmmsg(struct trace_event_raw_sys_exit *ctx)   { return exit_rw(ctx, DIR_WRITE); }

// sendfile64: args[0] = out_fd (socket), args[1] = in_fd (file)
SEC("tracepoint/syscalls/sys_enter_sendfile64")
int tp_enter_sendfile64(struct trace_event_raw_sys_enter *ctx) { return enter_rw(ctx); }
SEC("tracepoint/syscalls/sys_exit_sendfile64")
int tp_exit_sendfile64(struct trace_event_raw_sys_exit *ctx)   { return exit_rw(ctx, DIR_WRITE); }

// ──────────────────────────────────────────────────────────────
// NETWORK RECV syscalls
// ──────────────────────────────────────────────────────────────

SEC("tracepoint/syscalls/sys_enter_recvfrom")
int tp_enter_recvfrom(struct trace_event_raw_sys_enter *ctx) { return enter_rw(ctx); }
SEC("tracepoint/syscalls/sys_exit_recvfrom")
int tp_exit_recvfrom(struct trace_event_raw_sys_exit *ctx)   { return exit_rw(ctx, DIR_READ); }

SEC("tracepoint/syscalls/sys_enter_recvmsg")
int tp_enter_recvmsg(struct trace_event_raw_sys_enter *ctx) { return enter_rw(ctx); }
SEC("tracepoint/syscalls/sys_exit_recvmsg")
int tp_exit_recvmsg(struct trace_event_raw_sys_exit *ctx)   { return exit_rw(ctx, DIR_READ); }

SEC("tracepoint/syscalls/sys_enter_recvmmsg")
int tp_enter_recvmmsg(struct trace_event_raw_sys_enter *ctx) { return enter_rw(ctx); }
SEC("tracepoint/syscalls/sys_exit_recvmmsg")
int tp_exit_recvmmsg(struct trace_event_raw_sys_exit *ctx)   { return exit_rw(ctx, DIR_READ); }

// ──────────────────────────────────────────────────────────────
// FD CREATION – socket
// ──────────────────────────────────────────────────────────────

SEC("tracepoint/syscalls/sys_exit_socket")
int tp_exit_socket(struct trace_event_raw_sys_exit *ctx)
{ return exit_fd_open(ctx, FD_CLASS_SOCKET); }

SEC("tracepoint/syscalls/sys_exit_accept")
int tp_exit_accept(struct trace_event_raw_sys_exit *ctx)
{ return exit_fd_open(ctx, FD_CLASS_SOCKET); }

SEC("tracepoint/syscalls/sys_exit_accept4")
int tp_exit_accept4(struct trace_event_raw_sys_exit *ctx)
{ return exit_fd_open(ctx, FD_CLASS_SOCKET); }

// ──────────────────────────────────────────────────────────────
// FD CREATION – regular file
// ──────────────────────────────────────────────────────────────

SEC("tracepoint/syscalls/sys_exit_open")
int tp_exit_open(struct trace_event_raw_sys_exit *ctx)
{ return exit_fd_open(ctx, FD_CLASS_FILE); }

SEC("tracepoint/syscalls/sys_exit_openat")
int tp_exit_openat(struct trace_event_raw_sys_exit *ctx)
{ return exit_fd_open(ctx, FD_CLASS_FILE); }

SEC("tracepoint/syscalls/sys_exit_creat")
int tp_exit_creat(struct trace_event_raw_sys_exit *ctx)
{ return exit_fd_open(ctx, FD_CLASS_FILE); }

// ──────────────────────────────────────────────────────────────
// FD CREATION – pipe
// pipe/pipe2 return 0 and write the two FDs into a user-space array.
// We save the pointer on enter and read both FDs on exit.
// ──────────────────────────────────────────────────────────────

SEC("tracepoint/syscalls/sys_enter_pipe")
int tp_enter_pipe(struct trace_event_raw_sys_enter *ctx)
{
    __u64 ptgid = bpf_get_current_pid_tgid();
    __u32 tid   = (__u32)(ptgid & 0xFFFFFFFF);
    __u64 ptr   = (__u64)ctx->args[0];
    bpf_map_update_elem(&tid_pipeptr_map, &tid, &ptr, BPF_ANY);
    return 0;
}

SEC("tracepoint/syscalls/sys_exit_pipe")
int tp_exit_pipe(struct trace_event_raw_sys_exit *ctx)
{
    if ((__s32)ctx->ret != 0) return 0;

    __u64 ptgid = bpf_get_current_pid_tgid();
    __u32 tgid  = (__u32)(ptgid >> 32);
    __u32 tid   = (__u32)(ptgid & 0xFFFFFFFF);

    __u64 *ptrp = bpf_map_lookup_elem(&tid_pipeptr_map, &tid);
    if (!ptrp) return 0;
    bpf_map_delete_elem(&tid_pipeptr_map, &tid);

    int fds[2] = {};
    bpf_probe_read_user(fds, sizeof(fds), (void *)(long)*ptrp);

    __u8 cls = FD_CLASS_PIPE;
    set_fd_class(tgid, (__u32)fds[0], cls);
    set_fd_class(tgid, (__u32)fds[1], cls);
    emit_fd_event(ctx, tgid, tid, (__u32)fds[0], cls, FD_OP_OPEN);
    emit_fd_event(ctx, tgid, tid, (__u32)fds[1], cls, FD_OP_OPEN);
    return 0;
}

SEC("tracepoint/syscalls/sys_enter_pipe2")
int tp_enter_pipe2(struct trace_event_raw_sys_enter *ctx)
{
    __u64 ptgid = bpf_get_current_pid_tgid();
    __u32 tid   = (__u32)(ptgid & 0xFFFFFFFF);
    __u64 ptr   = (__u64)ctx->args[0];
    bpf_map_update_elem(&tid_pipeptr_map, &tid, &ptr, BPF_ANY);
    return 0;
}

SEC("tracepoint/syscalls/sys_exit_pipe2")
int tp_exit_pipe2(struct trace_event_raw_sys_exit *ctx)
{
    if ((__s32)ctx->ret != 0) return 0;

    __u64 ptgid = bpf_get_current_pid_tgid();
    __u32 tgid  = (__u32)(ptgid >> 32);
    __u32 tid   = (__u32)(ptgid & 0xFFFFFFFF);

    __u64 *ptrp = bpf_map_lookup_elem(&tid_pipeptr_map, &tid);
    if (!ptrp) return 0;
    bpf_map_delete_elem(&tid_pipeptr_map, &tid);

    int fds[2] = {};
    bpf_probe_read_user(fds, sizeof(fds), (void *)(long)*ptrp);

    __u8 cls = FD_CLASS_PIPE;
    set_fd_class(tgid, (__u32)fds[0], cls);
    set_fd_class(tgid, (__u32)fds[1], cls);
    emit_fd_event(ctx, tgid, tid, (__u32)fds[0], cls, FD_OP_OPEN);
    emit_fd_event(ctx, tgid, tid, (__u32)fds[1], cls, FD_OP_OPEN);
    return 0;
}

// ──────────────────────────────────────────────────────────────
// FD DUPLICATION – dup/dup2/dup3 inherit the class of the old fd.
// dup(oldfd)       → newfd = ret
// dup2(oldfd,newfd)→ newfd = ret
// dup3(oldfd,newfd,flags) → newfd = ret
// In all cases args[0] = oldfd, ret = newfd.
// ──────────────────────────────────────────────────────────────

SEC("tracepoint/syscalls/sys_enter_dup")
int tp_enter_dup(struct trace_event_raw_sys_enter *ctx)
{
    // Save old fd so exit can look it up.
    __u64 ptgid = bpf_get_current_pid_tgid();
    __u32 tid   = (__u32)(ptgid & 0xFFFFFFFF);
    __u32 oldfd = (__u32)ctx->args[0];
    save_fd_for_tid(tid, oldfd);
    return 0;
}

static __always_inline int exit_dup(struct trace_event_raw_sys_exit *ctx)
{
    __s32 ret = (__s32)ctx->ret;
    if (ret < 0) {
        __u64 ptgid = bpf_get_current_pid_tgid();
        pop_fd_for_tid((__u32)(ptgid & 0xFFFFFFFF));
        return 0;
    }
    __u64 ptgid = bpf_get_current_pid_tgid();
    __u32 tgid  = (__u32)(ptgid >> 32);
    __u32 tid   = (__u32)(ptgid & 0xFFFFFFFF);
    __u32 oldfd = pop_fd_for_tid(tid);
    __u32 newfd = (__u32)ret;
    __u8 cls    = get_fd_class(tgid, oldfd);
    if (cls != FD_CLASS_UNKNOWN) {
        set_fd_class(tgid, newfd, cls);
        emit_fd_event(ctx, tgid, tid, newfd, cls, FD_OP_OPEN);
    }
    return 0;
}

SEC("tracepoint/syscalls/sys_exit_dup")
int tp_exit_dup(struct trace_event_raw_sys_exit *ctx) { return exit_dup(ctx); }

SEC("tracepoint/syscalls/sys_enter_dup2")
int tp_enter_dup2(struct trace_event_raw_sys_enter *ctx)
{
    __u64 ptgid = bpf_get_current_pid_tgid();
    __u32 tid   = (__u32)(ptgid & 0xFFFFFFFF);
    __u32 oldfd = (__u32)ctx->args[0];
    save_fd_for_tid(tid, oldfd);
    return 0;
}
SEC("tracepoint/syscalls/sys_exit_dup2")
int tp_exit_dup2(struct trace_event_raw_sys_exit *ctx) { return exit_dup(ctx); }

SEC("tracepoint/syscalls/sys_enter_dup3")
int tp_enter_dup3(struct trace_event_raw_sys_enter *ctx)
{
    __u64 ptgid = bpf_get_current_pid_tgid();
    __u32 tid   = (__u32)(ptgid & 0xFFFFFFFF);
    __u32 oldfd = (__u32)ctx->args[0];
    save_fd_for_tid(tid, oldfd);
    return 0;
}
SEC("tracepoint/syscalls/sys_exit_dup3")
int tp_exit_dup3(struct trace_event_raw_sys_exit *ctx) { return exit_dup(ctx); }

// ──────────────────────────────────────────────────────────────
// FD CLOSE
// ──────────────────────────────────────────────────────────────

SEC("tracepoint/syscalls/sys_enter_close")
int tp_enter_close(struct trace_event_raw_sys_enter *ctx)
{
    __u64 ptgid = bpf_get_current_pid_tgid();
    __u32 tgid  = (__u32)(ptgid >> 32);
    __u32 tid   = (__u32)(ptgid & 0xFFFFFFFF);
    __u32 fd    = (__u32)ctx->args[0];

    __u8 cls = get_fd_class(tgid, fd);
    del_fd_class(tgid, fd);
    if (cls != FD_CLASS_UNKNOWN) {
        emit_fd_event(ctx, tgid, tid, fd, cls, FD_OP_CLOSE);
    }
    return 0;
}

// ──────────────────────────────────────────────────────────────
// PROCESS LIFECYCLE
// ──────────────────────────────────────────────────────────────

SEC("tracepoint/sched/sched_process_exit")
int tp_sched_exit(void *ctx)
{
    __u64 ptgid = bpf_get_current_pid_tgid();
    __u32 tgid  = (__u32)(ptgid >> 32);
    __u32 tid   = (__u32)(ptgid & 0xFFFFFFFF);
    // Only emit exit event for the main thread (process exit).
    if (tid != tgid) return 0;
    emit_proc_event(ctx, tgid, tid, PROC_OP_EXIT);
    return 0;
}

SEC("tracepoint/sched/sched_process_fork")
int tp_sched_fork(void *ctx)
{
    __u64 ptgid = bpf_get_current_pid_tgid();
    __u32 tgid  = (__u32)(ptgid >> 32);
    __u32 tid   = (__u32)(ptgid & 0xFFFFFFFF);
    emit_proc_event(ctx, tgid, tid, PROC_OP_FORK);
    return 0;
}

char LICENSE[] SEC("license") = "GPL";
