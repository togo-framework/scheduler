# scheduler — docs

The `togo-framework/scheduler` plugin runs recurring jobs on a cron schedule,
dispatched over the togo queue.

- **Install:** `togo install togo-framework/scheduler`
- **Marketplace:** https://to-go.dev/plugins/scheduler
- **Source:** https://github.com/togo-framework/scheduler

## Contents
- [Usage](usage.md) — registering jobs, scheduling, cadences, managing runs

## Overview
Register a named handler with `scheduler.Register`, then schedule it with a cron
expression or a fluent helper via the kernel service (`scheduler.FromKernel`).
Jobs are dispatched onto the kernel Queue (so they run on a worker), with
overlap prevention and recorded run history. See [usage.md](usage.md).
