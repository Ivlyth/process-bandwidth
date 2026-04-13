/* Minimal BPF helper declarations for eBPF programs.
 * Modelled after libbpf's bpf_helpers.h.
 */
#pragma once

#include "vmlinux.h"
#include <linux/bpf.h>

/* Shorthand for BPF program/map section annotations */
#ifndef SEC
#define SEC(NAME) __attribute__((section(NAME), used))
#endif

/* always_inline helper */
#ifndef __always_inline
#define __always_inline inline __attribute__((always_inline))
#endif

/* BPF map definition macros (BTF-style, supported since clang 10+) */
#define __uint(name, val)  int (*name)[val]
#define __type(name, val)  typeof(val) *name
#define __array(name, val) typeof(val) *name[]

/* BPF helper function declarations.
 * These are resolved by the BPF verifier to built-in kernel helpers.
 */

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

/* BPF_F_CURRENT_CPU: use current CPU index as perf event map key */
#ifndef BPF_F_CURRENT_CPU
#define BPF_F_CURRENT_CPU ((__u64)-1)
#endif

/* Convenience zero-init macro */
#define __builtin_memset(d, b, n) __builtin_memset((d), (b), (n))
