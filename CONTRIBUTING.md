# Contributing to KernelView

First off, thank you for considering contributing to KernelView! It's people like you that make KernelView such a great tool.

KernelView is an eBPF-powered Kubernetes autopilot. It requires knowledge across several domains: Go, eBPF/C, Kubernetes internals, and React/TypeScript. Don't worry if you aren't an expert in all of these! We welcome contributions to any part of the stack.

## Table of Contents
1. [Architecture Overview](#architecture-overview)
2. [Development Environment Setup](#development-environment-setup)
3. [Component Guidelines](#component-guidelines)
4. [Pull Request Process](#pull-request-process)
5. [Adding New Incident Types (v2 Spec)](#adding-new-incident-types)

## Architecture Overview

KernelView operates on an Open-Core model:
- **Open Source (Apache 2.0)**: eBPF Agent, Collector (Metrics/Ingestion), Dashboard.
- **Enterprise (Closed)**: AI Correlator, Remediation Operator (Safety Engine).

This repository contains the Open Source components and the interfaces for the Enterprise components.

## Development Environment Setup

### Prerequisites
- Linux kernel 5.8+ (for advanced eBPF features)
- Go 1.22+
- Clang/LLVM 12+ (for compiling eBPF C code)
- Node.js 20+ (for Dashboard)
- Docker & kind (Kubernetes IN Docker) for local testing

### Quick Start
1. Clone the repository: `git clone https://github.com/kernelview/kernelview.git`
2. Compile the eBPF programs: `make bpf`
3. Build the Go binaries: `make build`
4. Run the local test cluster: `make kind-up`
5. Deploy KernelView to the local cluster: `make deploy-local`

## Component Guidelines

### 1. eBPF C Code (`/bpf`)
- Must compile cleanly with Clang `-target bpf`.
- Use the provided headers in `bpf/headers/` (`helpers.h`, `maps.h`).
- Keep maps minimal to reduce memory footprint. Prefer per-CPU maps for high-frequency events.
- Strictly adhere to the GPL compatibility rules if using GPL-only BPF helpers.

### 2. Go Backend (`/cmd`, `/internal`, `/pkg`)
- Follow standard formatting (`gofmt`, `goimports`).
- Write comprehensive unit tests (`go test -v ./...`).
- When adding new metrics, register them in `pkg/models` to ensure both VictoriaMetrics and BadgerDB compatibility.
- Ensure the agent runs securely without `--privileged` (rely on `CAP_BPF`, `CAP_PERFMON`, `CAP_NET_ADMIN`).

### 3. Dashboard (`/dashboard`)
- Built with Vite + React + TypeScript + Tailwind CSS + shadcn/ui.
- Keep the aesthetic aligned: dark mode first, Inter/JetBrains Mono fonts, glassmorphism.
- Avoid introducing large dependencies; prefer D3.js and Recharts for custom visualizations.

## Pull Request Process
1. Fork the repo and create your branch from `main`.
2. If you've changed APIs, update the protobuf definitions (`make proto`).
3. If you've changed eBPF code, ensure `make bpf` passes and commit the resulting object files (if applicable to the workflow).
4. Run the test suite: `make test`.
5. Ensure your code is properly formatted: `make format`.
6. Issue that PR! A maintainer will review it. We enforce DCO (Developer Certificate of Origin), so please sign-off your commits (`git commit -s`).

## Adding New Incident Types
KernelView implements a dynamic incident routing architecture. To add a new incident pattern:
1. Define the signal in eBPF (`/bpf`) or the Collector (`/internal/collector`).
2. Add the `IncidentType` constant to `/internal/collector/classifier/types.go` and register it.
3. Add classification logic to the appropriate family in `/internal/collector/classifier/`.
4. Add the prompt template in `/internal/correlator/prompts/` and register it in `builder.go`.
