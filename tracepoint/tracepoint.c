#include "ecapture.h"

#define DIRECTION_WRITE 0
#define DIRECTION_READ 1

//const u8 FD_TYPE_SOCKET = 1;
//const u8 FD_TYPE_FILE = 2;

#define FD_TYPE_SOCKET 1
#define FD_TYPE_FILE 2

const u32 EVENT_TYPE_RW = 0;
const u32 EVENT_TYPE_CLOSE = 1;
const u32 EVENT_TYPE_EXIT = 2;

// pfid_type_map 用于存储 tid+fd 与 fd-type 的映射关系
typedef struct {
    u32 pid;
    u32 fd;
} pfid_t;

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, pfid_t);
    __type(value, u8);
    __uint(max_entries, 65536);
} pfid_type_map SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, u32);  // pid
    __type(value, u32);  // fd
    __uint(max_entries, 65536);
} pid_fid_map SEC(".maps");

struct exit_event_t {
    u8  type;
    u32 pid;
    u32 tid;
} __attribute__((packed));

struct close_event_t {
    u8  type;
    u32 pid;
    u32 tid;
    u32 fd;
} __attribute__((packed));

struct rw_event_t {
    u8  type;
    u32 pid;
    u32 tid;
    u32 fd;
    u8 fdtype;
    u8 direction;  // 0-write, 1-read
    u64 size;
} __attribute__((packed));

struct {
    __uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
     __uint(max_entries, 65536);
} bp_events SEC(".maps");


static inline __attribute__((__always_inline__)) void save_fd_with_pid(char * source, struct trace_event_raw_sys_enter* ctx) {
    u64 current_pid_tgid = bpf_get_current_pid_tgid();
    u32 pid = current_pid_tgid & 0xffffffff;
    u32 tgid = current_pid_tgid >> 32;

    u32 fd = (u32)ctx->args[0];

    long uret = bpf_map_update_elem(&pid_fid_map, &pid, &fd, BPF_ANY);
    if (uret != 0) {
        debug_bpf_printk("%s: ERROR: insert elem to pid_fid_map failed (tgid:%u, fd:%u)\n", source, tgid, fd);
    } else {
//        debug_bpf_printk("%s: OK: insert elem to pid_fid_map ok (tgid:%u, fd:%u)\n", source, tgid, fd);
    }
}


static inline __attribute__((__always_inline__)) void save_fdtype_with_pfid(char *source, struct trace_event_raw_sys_exit* ctx, u8 fdtype) {
    u64 current_pid_tgid = bpf_get_current_pid_tgid();
    u32 pid = current_pid_tgid & 0xffffffff;
    u32 tgid = current_pid_tgid >> 32;

    s32 ret = (s32)ctx->ret;
    if (ret < 0) {
        return;
    }
    u32 fd = (u32)ret;

    pfid_t pfid;
    __builtin_memset(&pfid, 0, sizeof(pfid));
    pfid.pid = pid;
    pfid.fd = fd;

    long uret = bpf_map_update_elem(&pfid_type_map, &pfid, &fdtype, BPF_ANY);  // socket type fd
    if (uret != 0) {
        debug_bpf_printk("%s: ERROR: insert socket fd to pfid_type_map failed (tgid:%u, fd:%u)\n", source, tgid, fd);
    } else {
//        debug_bpf_printk("%s: OK: insert socket fd to pfid_type_map ok (tgid:%u, fd:%u)\n", source, tgid, fd);
    }
}

static inline __attribute__((__always_inline__)) void gen_rw_event(char *source, struct trace_event_raw_sys_exit* ctx, u8 direction) {
    u64 current_pid_tgid = bpf_get_current_pid_tgid();
    u32 tgid = current_pid_tgid >> 32;
    u32 pid = current_pid_tgid & 0xffffffff;

    u32 *fdp = bpf_map_lookup_elem(&pid_fid_map, &pid);
    if (fdp == NULL) {
        debug_bpf_printk("%s: ERROR: no fd found for tgid: %u\n", source, tgid);
        return;
    }

    u32 fd = *fdp;

    long udret = bpf_map_delete_elem(&pid_fid_map, &pid);
    if (udret != 0) {
        debug_bpf_printk("%s: ERROR: delete elem from pid_fid_map failed (tgid:%u, fd:%u)\n", source, tgid, fd);
    } else {
//        debug_bpf_printk("%s: OK: delete elem from pid_fid_map ok (tgid:%u, fd:%u)\n", source, tgid, fd);
    }

    if (fd <= 2) {
        return;
    }

    s64 ret = ctx->ret;
    if (ret <= 0) {
        return;
    }

    pfid_t pfid;
    __builtin_memset(&pfid, 0, sizeof(pfid));
    pfid.pid = pid;
    pfid.fd = fd;

    u8 *fdtp = bpf_map_lookup_elem(&pfid_type_map, &pfid);
    u8 fdt = 0;

    if (fdtp != NULL) {
        fdt = *fdtp;
    }

    if (fdt == FD_TYPE_FILE) {  // file related fd
//        debug_bpf_printk("%s: don't output event log because of file type fd, tgid:%u, fd:%u\n", source, tgid, fd);
        return;
    }

    struct rw_event_t rw_event;
    __builtin_memset(&rw_event, 0, sizeof(rw_event));
    rw_event.type = EVENT_TYPE_RW;
    rw_event.pid = tgid;
    rw_event.tid = pid;
    rw_event.fd = fd;
    rw_event.fdtype = fdt;
    rw_event.direction = direction;
    rw_event.size = (u64)ret;

    long udret2 = bpf_perf_event_output(ctx, &bp_events, BPF_F_CURRENT_CPU, &rw_event, sizeof(rw_event));
    if (udret2 != 0) {
        debug_bpf_printk("%s: ERROR: failed output read/write log, fd:%u, return value:%d\n", source, rw_event.fd, udret2);
    } else {
//        debug_bpf_printk("%s: OK: success output read/write log, fd:%u, read size %llu\n", source, rw_event.fd, rw_event.size);
    }
}

static inline __attribute__((__always_inline__)) void gen_exit_event(char *source, void* ctx) {
u64 current_pid_tgid = bpf_get_current_pid_tgid();
    u32 pid = current_pid_tgid & 0xffffffff;
    u32 tgid = current_pid_tgid >> 32;

    struct exit_event_t exit_event;
    __builtin_memset(&exit_event, 0, sizeof(exit_event));
    exit_event.type = EVENT_TYPE_EXIT;
    exit_event.pid = tgid;
    exit_event.tid = pid;

    long udret2 = bpf_perf_event_output(ctx, &bp_events, BPF_F_CURRENT_CPU, &exit_event, sizeof(exit_event));
    if (udret2 != 0) {
        debug_bpf_printk("%s: ERROR: failed output exit log, tgid:%u, return value:%d\n", source, tgid, udret2);
    } else {
        //  debug_bpf_printk("%s: OK: success output exit log, pid:%u, tid:%u\n", source, pid, tgid);
    }
}

SEC("tracepoint/syscalls/sys_enter_write")
int tracepoint_sys_enter_write(struct trace_event_raw_sys_enter* ctx) {
    char source[] = "sys_enter_write";
    save_fd_with_pid(source, ctx);
    return 0;
}

SEC("tracepoint/syscalls/sys_exit_write")
int tracepoint_sys_exit_write(struct trace_event_raw_sys_exit* ctx) {
    char source[] = "sys_exit_write";
    gen_rw_event(source, ctx, DIRECTION_WRITE);
    return 0;
}

SEC("tracepoint/syscalls/sys_enter_pwrite")
int tracepoint_sys_enter_pwrite(struct trace_event_raw_sys_enter* ctx) {
    char source[] = "sys_enter_pwrite";
    save_fd_with_pid(source, ctx);
    return 0;
}

SEC("tracepoint/syscalls/sys_exit_pwrite")
int tracepoint_sys_exit_pwrite(struct trace_event_raw_sys_exit* ctx) {
    char source[] = "sys_exit_pwrite";
    gen_rw_event(source, ctx, DIRECTION_READ);
    return 0;
}

SEC("tracepoint/syscalls/sys_enter_pwritev")
int tracepoint_sys_enter_pwritev(struct trace_event_raw_sys_enter* ctx) {
    char source[] = "sys_enter_pwritev";
    save_fd_with_pid(source, ctx);
    return 0;
}

SEC("tracepoint/syscalls/sys_exit_pwritev")
int tracepoint_sys_exit_pwritev(struct trace_event_raw_sys_exit* ctx) {
    char source[] = "sys_exit_pwritev";
    gen_rw_event(source, ctx, DIRECTION_READ);
    return 0;
}

SEC("tracepoint/syscalls/sys_enter_pwrite64")
int tracepoint_sys_enter_pwrite64(struct trace_event_raw_sys_enter* ctx) {
    char source[] = "sys_enter_pwrite64";
    save_fd_with_pid(source, ctx);
    return 0;
}

SEC("tracepoint/syscalls/sys_exit_pwrite64")
int tracepoint_sys_exit_pwrite64(struct trace_event_raw_sys_exit* ctx) {
    char source[] = "sys_exit_pwrite64";
    gen_rw_event(source, ctx, DIRECTION_READ);
    return 0;
}

SEC("tracepoint/syscalls/sys_enter_sendto")
int tracepoint_sys_enter_sendto(struct trace_event_raw_sys_enter* ctx) {
    char source[] = "sys_enter_sendto";
    save_fd_with_pid(source, ctx);
    return 0;
}

SEC("tracepoint/syscalls/sys_exit_sendto")
int tracepoint_sys_exit_sendto(struct trace_event_raw_sys_exit* ctx) {
    char source[] = "sys_exit_sendto";
    gen_rw_event(source, ctx, DIRECTION_WRITE);
    return 0;
}

SEC("tracepoint/syscalls/sys_enter_sendmsg")
int tracepoint_sys_enter_sendmsg(struct trace_event_raw_sys_enter* ctx) {
    char source[] = "sys_enter_sendmsg";
    save_fd_with_pid(source, ctx);
    return 0;
}

SEC("tracepoint/syscalls/sys_exit_sendmsg")
int tracepoint_sys_exit_sendmsg(struct trace_event_raw_sys_exit* ctx) {
    char source[] = "sys_exit_sendmsg";
    gen_rw_event(source, ctx, DIRECTION_WRITE);
    return 0;
}

SEC("tracepoint/syscalls/sys_enter_sendmmsg")
int tracepoint_sys_enter_sendmmsg(struct trace_event_raw_sys_enter* ctx) {
    char source[] = "sys_enter_sendmmsg";
    save_fd_with_pid(source, ctx);
    return 0;
}

SEC("tracepoint/syscalls/sys_exit_sendmmsg")
int tracepoint_sys_exit_sendmmsg(struct trace_event_raw_sys_exit* ctx) {
    char source[] = "sys_exit_sendmmsg";
    gen_rw_event(source, ctx, DIRECTION_WRITE);
    return 0;
}

SEC("tracepoint/syscalls/sys_enter_sendfile64")
int tracepoint_sys_enter_sendfile64(struct trace_event_raw_sys_enter* ctx) {
    char source[] = "sys_enter_sendfile64";
    save_fd_with_pid(source, ctx);
    return 0;
}

SEC("tracepoint/syscalls/sys_exit_sendfile64")
int tracepoint_sys_exit_sendfile64(struct trace_event_raw_sys_exit* ctx) {
    char source[] = "sys_exit_sendfile64";
    gen_rw_event(source, ctx, DIRECTION_WRITE);
    return 0;
}

SEC("tracepoint/syscalls/sys_enter_read")
int tracepoint_sys_enter_read(struct trace_event_raw_sys_enter* ctx) {
    char source[] = "sys_enter_read";
    save_fd_with_pid(source, ctx);
    return 0;
}

SEC("tracepoint/syscalls/sys_exit_read")
int tracepoint_sys_exit_read(struct trace_event_raw_sys_exit* ctx) {
    char source[] = "sys_exit_read";
    gen_rw_event(source, ctx, DIRECTION_READ);
    return 0;
}

SEC("tracepoint/syscalls/sys_enter_pread")
int tracepoint_sys_enter_pread(struct trace_event_raw_sys_enter* ctx) {
    char source[] = "sys_enter_pread";
    save_fd_with_pid(source, ctx);
    return 0;
}

SEC("tracepoint/syscalls/sys_exit_pread")
int tracepoint_sys_exit_pread(struct trace_event_raw_sys_exit* ctx) {
    char source[] = "sys_exit_pread";
    gen_rw_event(source, ctx, DIRECTION_READ);
    return 0;
}

SEC("tracepoint/syscalls/sys_enter_preadv")
int tracepoint_sys_enter_preadv(struct trace_event_raw_sys_enter* ctx) {
    char source[] = "sys_enter_preadv";
    save_fd_with_pid(source, ctx);
    return 0;
}

SEC("tracepoint/syscalls/sys_exit_preadv")
int tracepoint_sys_exit_preadv(struct trace_event_raw_sys_exit* ctx) {
    char source[] = "sys_exit_preadv";
    gen_rw_event(source, ctx, DIRECTION_READ);
    return 0;
}

SEC("tracepoint/syscalls/sys_enter_pread64")
int tracepoint_sys_enter_pread64(struct trace_event_raw_sys_enter* ctx) {
    char source[] = "sys_enter_pread64";
    save_fd_with_pid(source, ctx);
    return 0;
}

SEC("tracepoint/syscalls/sys_exit_pread64")
int tracepoint_sys_exit_pread64(struct trace_event_raw_sys_exit* ctx) {
    char source[] = "sys_exit_pread64";
    gen_rw_event(source, ctx, DIRECTION_READ);
    return 0;
}

SEC("tracepoint/syscalls/sys_enter_recvfrom")
int tracepoint_sys_enter_recvfrom(struct trace_event_raw_sys_enter* ctx) {
    char source[] = "sys_enter_recvfrom";
    save_fd_with_pid(source, ctx);
    return 0;
}

SEC("tracepoint/syscalls/sys_exit_recvfrom")
int tracepoint_sys_exit_recvfrom(struct trace_event_raw_sys_exit* ctx) {
    char source[] = "sys_exit_recvfrom";
    gen_rw_event(source, ctx, DIRECTION_READ);
    return 0;
}

SEC("tracepoint/syscalls/sys_enter_recvmsg")
int tracepoint_sys_enter_recvmsg(struct trace_event_raw_sys_enter* ctx) {
    char source[] = "sys_enter_recvmsg";
    save_fd_with_pid(source, ctx);
    return 0;
}

SEC("tracepoint/syscalls/sys_exit_recvmsg")
int tracepoint_sys_exit_recvmsg(struct trace_event_raw_sys_exit* ctx) {
    char source[] = "sys_exit_recvmsg";
    gen_rw_event(source, ctx, DIRECTION_READ);
    return 0;
}

SEC("tracepoint/syscalls/sys_enter_recvmmsg")
int tracepoint_sys_enter_recvmmsg(struct trace_event_raw_sys_enter* ctx) {
    char source[] = "sys_enter_recvmmsg";
    save_fd_with_pid(source, ctx);
    return 0;
}

SEC("tracepoint/syscalls/sys_exit_recvmmsg")
int tracepoint_sys_exit_recvmmsg(struct trace_event_raw_sys_exit* ctx) {
    char source[] = "sys_exit_recvmmsg";
    gen_rw_event(source, ctx, DIRECTION_READ);
    return 0;
}

// ---------------------------------------- where new fds comes from
SEC("tracepoint/syscalls/sys_exit_accept4")
int tracepoint_sys_exit_accept4(struct trace_event_raw_sys_exit* ctx) {
    char source[] = "sys_exit_accept4";
    save_fdtype_with_pfid(source, ctx, FD_TYPE_SOCKET);
    return 0;
}

SEC("tracepoint/syscalls/sys_exit_accept")
int tracepoint_sys_exit_accept(struct trace_event_raw_sys_exit* ctx) {
    char source[] = "sys_exit_accept";
    save_fdtype_with_pfid(source, ctx, FD_TYPE_SOCKET);
    return 0;
}

SEC("tracepoint/syscalls/sys_exit_socket")
int tracepoint_sys_exit_socket(struct trace_event_raw_sys_exit* ctx) {
    char source[] = "sys_exit_socket";
    save_fdtype_with_pfid(source, ctx, FD_TYPE_SOCKET);
    return 0;
}

SEC("tracepoint/syscalls/sys_exit_open")
int tracepoint_sys_exit_open(struct trace_event_raw_sys_exit* ctx) {
    char source[] = "sys_exit_open";
    save_fdtype_with_pfid(source, ctx, FD_TYPE_FILE);
    return 0;
}

SEC("tracepoint/syscalls/sys_exit_openat")
int tracepoint_sys_exit_openat(struct trace_event_raw_sys_exit* ctx) {
    char source[] = "sys_exit_openat";
    save_fdtype_with_pfid(source, ctx, FD_TYPE_FILE);
    return 0;
}

SEC("tracepoint/syscalls/sys_exit_creat")
int tracepoint_sys_exit_creat(struct trace_event_raw_sys_exit* ctx) {
    char source[] = "sys_exit_creat";
    save_fdtype_with_pfid(source, ctx, FD_TYPE_FILE);
    return 0;
}

SEC("tracepoint/syscalls/sys_enter_close")
int tracepoint_sys_enter_close(struct trace_event_raw_sys_enter* ctx) {

    u64 current_pid_tgid = bpf_get_current_pid_tgid();
    u32 pid = current_pid_tgid & 0xffffffff;
    u32 tgid = current_pid_tgid >> 32;

    u32 fd = (u32)ctx->args[0];

    pfid_t pfid;
    __builtin_memset(&pfid, 0, sizeof(pfid));
    pfid.pid = pid;
    pfid.fd = fd;

    long uret = bpf_map_delete_elem(&pfid_type_map, &pfid);
    if (uret != 0) {
        debug_bpf_printk("tracepoint/sys_enter_close: ERROR: delete from pfid_type_map failed (pid:%u, tgid:%u, fd:%u)\n", pid, tgid, fd);
    } else {
//        debug_bpf_printk("tracepoint/sys_enter_close: OK: delete from pfid_type_map ok (pid:%u, tgid:%u, fd:%u)\n", pid, tgid, fd);
    }

    struct close_event_t close_event;
    __builtin_memset(&close_event, 0, sizeof(close_event));
    close_event.type = EVENT_TYPE_CLOSE;
    close_event.pid = tgid;
    close_event.tid = pid;
    close_event.fd = fd;

    long udret2 = bpf_perf_event_output(ctx, &bp_events, BPF_F_CURRENT_CPU, &close_event, sizeof(close_event));
    if (udret2 != 0) {
        debug_bpf_printk("%s: ERROR: failed output close log, fd:%u, return value:%d\n", source, close_event.fd, udret2);
    } else {
        //  debug_bpf_printk("%s: OK: success output close log, fd:%u\n", source, close_event.fd);
    }

    return 0;
}

SEC("tracepoint/sched/sched_process_exit")
int tracepoint_sched_process_exit(void* ctx) {
    char source[] = "sched_process_exit";
    gen_exit_event(source, ctx);
    return 0;
}

