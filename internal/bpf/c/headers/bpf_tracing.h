/* Minimal BPF tracing helpers for eBPF programs.
 * Provides the tracepoint context structs and PT_REGS helpers.
 */
#pragma once

#include "vmlinux.h"

/* Tracepoint raw context structures.
 * These match the kernel's internal layout and are stable across kernel versions.
 */

struct trace_entry {
    unsigned short type;
    unsigned char  flags;
    unsigned char  preempt_count;
    int            pid;
};

/* Context passed to syscall enter tracepoint programs */
struct trace_event_raw_sys_enter {
    struct trace_entry ent;
    long int           id;      /* syscall number */
    unsigned long      args[6]; /* syscall arguments */
    char               __data[0];
};

/* Context passed to syscall exit tracepoint programs */
struct trace_event_raw_sys_exit {
    struct trace_entry ent;
    long int           id;  /* syscall number */
    long int           ret; /* return value */
    char               __data[0];
};
