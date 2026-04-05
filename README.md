# gRPC Distributed Transaction Ledger

A distributed transaction ledger built with Go and gRPC, using a leader–worker architecture where a central leader coordinates atomic commits across multiple worker nodes.

---

## Architecture

```
┌─────────────┐         gRPC          ┌──────────────┐
│   Client    │ ──────────────────▶  │    Leader     │
│ (bin/client)│                       │ (bin/leader)  │
└─────────────┘                       │   :9000       │
                                      └──────┬────────┘
                                             │ gRPC (fan-out)
                          ┌──────────────────┼──────────────────┐
                          ▼                  ▼                  ▼
                   ┌────────────┐   ┌────────────┐   ┌────────────┐
                   │   node-1   │   │   node-2   │   │   node-3   │
                   │  :50051    │   │  :50052    │   │  :50053    │
                   └────────────┘   └────────────┘   └────────────┘
```

The **client** submits transactions to the **leader**, which fans them out to all registered **worker nodes** and waits for quorum acknowledgement before committing.

---

## Prerequisites

| Tool | Version | Install |
|------|---------|---------|
| Go | ≥ 1.22 | https://go.dev/dl |
| protoc | latest | https://grpc.io/docs/protoc-installation |
| protoc-gen-go | latest | `go install google.golang.org/protobuf/cmd/protoc-gen-go@latest` |
| protoc-gen-go-grpc | latest | `go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest` |

---

## Getting Started

### 1. Clone and enter the repo

```bash
git clone https://github.com/example/grpc-distributed-ledger.git
cd grpc-distributed-ledger
```

### 2. Generate protobuf stubs

> **Must be done before `go mod tidy`** — the `proto` package only exists after codegen.

```bash
protoc \
  --go_out=. --go_opt=paths=source_relative \
  --go-grpc_out=. --go-grpc_opt=paths=source_relative \
  proto/ledger.proto
```

This writes `proto/ledger.pb.go` and `proto/ledger_grpc.pb.go`.

### 3. Tidy dependencies

```bash
go mod tidy
```

### 4. Build all binaries

```bash
make build
# → bin/node, bin/leader, bin/client
```

---

## Running the System

### Option A — One-shot demo

Starts nodes, leader, and client automatically, then cleans up:

```bash
make demo
```

### Option B — Manual (separate terminals)

**Terminal 1 — worker nodes:**
```bash
make run-nodes
```

**Terminal 2 — leader** (after nodes are up):
```bash
make run-leader
```

**Terminal 3 — client:**
```bash
make run-client
```

---

## Makefile Reference

| Target | Description |
|--------|-------------|
| `make all` | deps → proto → build (full setup) |
| `make deps` | `go mod tidy` |
| `make proto` | Regenerate gRPC stubs from `proto/ledger.proto` |
| `make build` | Compile all three binaries into `bin/` |
| `make run-nodes` | Start node-1/2/3 in the background |
| `make run-leader` | Start the leader (foreground) |
| `make run-client` | Fire 12 test transactions at the leader |
| `make demo` | Full end-to-end demo in one command |
| `make clean` | Delete `bin/` |

---

## VS Code Tasks

A `.vscode/tasks.json` is included. Open the Command Palette (`Ctrl+Shift+P`) and choose **Tasks: Run Task** to access all targets. `Ctrl+Shift+B` runs the default **build** task.

---

## Project Structure

```
.
├── proto/
│   ├── ledger.proto          # Service & message definitions
│   ├── ledger.pb.go          # Generated (do not edit)
│   └── ledger_grpc.pb.go     # Generated (do not edit)
├── node/                     # Worker node implementation
├── client/                   # CLI transaction client
├── bin/                      # Compiled binaries (git-ignored)
├── go.mod
├── go.sum
├── main.go
├── Makefile
└── .vscode/
    └── tasks.json
```

---

## Common Issues

### `cannot find module providing package .../proto`

The proto package doesn't exist yet. Run `make proto` **before** `go mod tidy`. See [Getting Started](#getting-started).

### `go mod tidy` tries to fetch the module from GitHub

Your `go.mod` needs a `replace` directive to point Go at your local source:

```
replace github.com/example/grpc-distributed-ledger => ./
```

### `protoc: command not found`

Install `protoc` from https://grpc.io/docs/protoc-installation, then ensure the `protoc-gen-go` and `protoc-gen-go-grpc` plugins are on your `PATH` (they install to `$(go env GOPATH)/bin`).

---
