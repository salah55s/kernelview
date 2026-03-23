// SPDX-License-Identifier: Apache-2.0
// KernelView — Process Execution Tracking
//
// Hooks sched:sched_process_exec to detect new process execution
// within containers for security auditing.

#include "headers/maps.h"
#include "headers/helpers.h"

char LICENSE[] SEC("license") = "Dual BSD/GPL";

SEC("tracepoint/sched/sched_process_exec")
int tracepoint_sched_process_exec(struct trace_event_raw_sched_process_exec *ctx) {
    struct exec_event *evt = bpf_ringbuf_reserve(&events, sizeof(*evt), 0);
    if (!evt)
        return 0;

    __builtin_memset(evt, 0, sizeof(*evt));
    evt->timestamp_ns = bpf_ktime_get_ns();

    __u64 pid_tgid = bpf_get_current_pid_tgid();
    evt->pid = pid_tgid >> 32;
    evt->cgroup_id = get_cgroup_id();

    // Get current task info
    struct task_struct *task = (struct task_struct *)bpf_get_current_task();
    if (task) {
        evt->ppid = BPF_CORE_READ(task, real_parent, tgid);
        evt->uid = BPF_CORE_READ(task, cred, uid.val);
    }

    bpf_get_current_comm(&evt->comm, sizeof(evt->comm));

    // Read filename from tracepoint context
    unsigned short filename_offset = BPF_CORE_READ(ctx, __data_loc_filename) & 0xFFFF;
    bpf_probe_read_str(&evt->filename, sizeof(evt->filename),
                       (void *)ctx + filename_offset);

    bpf_ringbuf_submit(evt, 0);
    return 0;
}
