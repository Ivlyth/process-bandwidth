/* Minimal BPF helper declarations for eBPF programs.
 * Self-contained: does not include <linux/bpf.h> so it compiles on macOS
 * and other non-Linux hosts when cross-compiling with -target bpf.
 * Constants are taken verbatim from the Linux kernel include/uapi/linux/bpf.h.
 */
#pragma once

#include "vmlinux.h"

/* Shorthand for BPF program/map section annotations */
#ifndef SEC
#define SEC(NAME) __attribute__((section(NAME), used))
#endif

/* always_inline helper */
#ifndef __always_inline
#define __always_inline inline __attribute__((always_inline))
#endif

/* ──────────────────────────────────────────────────────────────
 * BPF map types (enum bpf_map_type from linux/bpf.h)
 * ────────────────────────────────────────────────────────────── */
#define BPF_MAP_TYPE_UNSPEC                  0
#define BPF_MAP_TYPE_HASH                    1
#define BPF_MAP_TYPE_ARRAY                   2
#define BPF_MAP_TYPE_PROG_ARRAY              3
#define BPF_MAP_TYPE_PERF_EVENT_ARRAY        4
#define BPF_MAP_TYPE_PERCPU_HASH             5
#define BPF_MAP_TYPE_PERCPU_ARRAY            6
#define BPF_MAP_TYPE_STACK_TRACE             7
#define BPF_MAP_TYPE_CGROUP_ARRAY            8
#define BPF_MAP_TYPE_LRU_HASH                9
#define BPF_MAP_TYPE_LRU_PERCPU_HASH        10
#define BPF_MAP_TYPE_LPM_TRIE               11
#define BPF_MAP_TYPE_ARRAY_OF_MAPS          12
#define BPF_MAP_TYPE_HASH_OF_MAPS           13
#define BPF_MAP_TYPE_DEVMAP                 14
#define BPF_MAP_TYPE_SOCKMAP                15
#define BPF_MAP_TYPE_CPUMAP                 16
#define BPF_MAP_TYPE_XSKMAP                 17
#define BPF_MAP_TYPE_SOCKHASH               18
#define BPF_MAP_TYPE_CGROUP_STORAGE         19
#define BPF_MAP_TYPE_REUSEPORT_SOCKARRAY    20
#define BPF_MAP_TYPE_PERCPU_CGROUP_STORAGE  21
#define BPF_MAP_TYPE_QUEUE                  22
#define BPF_MAP_TYPE_STACK                  23
#define BPF_MAP_TYPE_SK_STORAGE             24
#define BPF_MAP_TYPE_DEVMAP_HASH            25
#define BPF_MAP_TYPE_STRUCT_OPS             26
#define BPF_MAP_TYPE_RINGBUF                27

/* ──────────────────────────────────────────────────────────────
 * BPF map update flags
 * ────────────────────────────────────────────────────────────── */
#define BPF_ANY     0U  /* create or update */
#define BPF_NOEXIST 1U  /* create only      */
#define BPF_EXIST   2U  /* update only      */

/* ──────────────────────────────────────────────────────────────
 * BPF_F_CURRENT_CPU: use current CPU index as perf event map key.
 * The kernel masks with 0xFFFFFFFF so (__u64)-1 == all-ones works.
 * ────────────────────────────────────────────────────────────── */
#ifndef BPF_F_CURRENT_CPU
#define BPF_F_CURRENT_CPU ((__u64)-1)
#endif

/* ──────────────────────────────────────────────────────────────
 * BPF helper function IDs (enum bpf_func_id from linux/bpf.h).
 * Only the helpers used by pbmon.c are listed here.
 * ────────────────────────────────────────────────────────────── */
#define BPF_FUNC_map_lookup_elem         1
#define BPF_FUNC_map_update_elem         2
#define BPF_FUNC_map_delete_elem         3
#define BPF_FUNC_trace_printk            6
#define BPF_FUNC_get_current_pid_tgid   14
#define BPF_FUNC_perf_event_output      25
#define BPF_FUNC_probe_read_user       112
#define BPF_FUNC_ringbuf_output        130

/* ──────────────────────────────────────────────────────────────
 * BPF map definition macros (BTF-style, supported since clang 10+)
 * ────────────────────────────────────────────────────────────── */
#define __uint(name, val)  int (*name)[val]
#define __type(name, val)  typeof(val) *name
#define __array(name, val) typeof(val) *name[]

/* ──────────────────────────────────────────────────────────────
 * BPF helper function declarations.
 * These are resolved by the BPF verifier to built-in kernel helpers.
 * ────────────────────────────────────────────────────────────── */

static u64 (*bpf_get_current_pid_tgid)(void) =
    (void *)BPF_FUNC_get_current_pid_tgid;

static long (*bpf_map_update_elem)(void *map, const void *key,
                                   const void *value, u64 flags) =
    (void *)BPF_FUNC_map_update_elem;

static void *(*bpf_map_lookup_elem)(void *map, const void *key) =
    (void *)BPF_FUNC_map_lookup_elem;

static long (*bpf_map_delete_elem)(void *map, const void *key) =
    (void *)BPF_FUNC_map_delete_elem;

static long (*bpf_perf_event_output)(void *ctx, void *map, u64 flags,
                                     void *data, u64 size) =
    (void *)BPF_FUNC_perf_event_output;

static long (*bpf_probe_read_user)(void *dst, u32 size,
                                   const void *unsafe_ptr) =
    (void *)BPF_FUNC_probe_read_user;

static long (*bpf_trace_printk)(const char *fmt, u32 fmt_size, ...) =
    (void *)BPF_FUNC_trace_printk;

/* bpf_ringbuf_output – only available on kernel >= 5.8 */
static long (*bpf_ringbuf_output)(void *ringbuf, void *data, u64 size,
                                  u64 flags) =
    (void *)BPF_FUNC_ringbuf_output;
