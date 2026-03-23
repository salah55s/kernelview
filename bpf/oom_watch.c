// SPDX-License-Identifier: Apache-2.0
// KernelView — OOM Kill Detection
//
// Hooks both kernel OOM killer (oom_kill_process) and cgroup OOM killer
// to capture all OOM events in containerized environments.

#include "headers/maps.h"
#include "headers/helpers.h"

char LICENSE[] SEC("license") = "Dual BSD/GPL";

// ============================================================
// Kernel OOM killer
// ============================================================

SEC("kprobe/oom_kill_process")
int kprobe_oom_kill_process(struct pt_regs *ctx) {
    struct oom_event *evt = bpf_ringbuf_reserve(&events, sizeof(*evt), 0);
    if (!evt)
        return 0;

    __builtin_memset(evt, 0, sizeof(*evt));
    evt->timestamp_ns = bpf_ktime_get_ns();
    evt->kill_source = 0; // kernel_oom

    // The first argument is the oom_control struct, second is the victim task
    struct task_struct *victim = (struct task_struct *)PT_REGS_PARM2(ctx);
    if (victim) {
        evt->victim_pid = BPF_CORE_READ(victim, tgid);
        BPF_CORE_READ_STR_INTO(&evt->victim_comm, victim, comm);
        evt->cgroup_id = BPF_CORE_READ(victim, cgroups, dfl_cgrp, kn, id);
    }

    bpf_ringbuf_submit(evt, 0);
    return 0;
}

// ============================================================
// Cgroup OOM killer (more common in containers)
// ============================================================

SEC("kprobe/cgroup_oom_kill")
int kprobe_cgroup_oom_kill(struct pt_regs *ctx) {
    struct oom_event *evt = bpf_ringbuf_reserve(&events, sizeof(*evt), 0);
    if (!evt)
        return 0;

    __builtin_memset(evt, 0, sizeof(*evt));
    evt->timestamp_ns = bpf_ktime_get_ns();
    evt->kill_source = 1; // cgroup_oom

    // For cgroup OOM, we capture from the current task context
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    evt->victim_pid = pid_tgid >> 32;
    bpf_get_current_comm(&evt->victim_comm, sizeof(evt->victim_comm));
    evt->cgroup_id = get_cgroup_id();

    bpf_ringbuf_submit(evt, 0);
    return 0;
}
