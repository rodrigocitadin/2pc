# Two-Phase Commit (2PC) with WAL & Recovery

This project is a functional implementation of the Two-Phase Commit (2PC) distributed consensus protocol, written entirely in Go. It demonstrates how distributed nodes can achieve atomic transaction commitment even in the presence of failures, using persistent storage and Write-Ahead Logging (WAL).

## What is Two-Phase Commit?

Two-Phase Commit is a standardized protocol for ensuring that a distributed transaction is either committed on all involved nodes or aborted on all of them. It consists of two phases:

1. **Prepare Phase**: The Coordinator asks all Participants if they are ready to commit. Participants lock resources and vote "Yes" or "No".
2. **Commit/Abort Phase**:
* If all Participants vote "Yes", the Coordinator sends a **Commit** message.
* If any Participant votes "No" (or times out), the Coordinator sends an **Abort** message.

This implementation also includes **Recovery** mechanisms. If a node crashes, it can rebuild its state and resolve pending transactions by reading its stable log upon restart.

## How This Project Implements 2PC

The implementation is divided into core library components (`internal/`) and storage engines (`internal/store/`):

### Node (`internal/node.go`): 

The core entity that acts as both a Coordinator and a Participant. It handles:

* Initiating transactions.
* Broadcasting `Prepare`, `Commit`, and `Abort` RPCs to peers.
* Handling incoming RPC requests via `NodeRPC`.
* Recovering state from disk on startup.


### Stable Store (`internal/store/stable.go`):

A persistent storage engine using Write-Ahead Logging (WAL).

* Records transaction states (`PREPARED`, `COMMITTED`, `ABORTED`) to disk before modifying volatile state.
* Uses `gob` encoding to save logs to `./logs/node_ID.wal`.
* Supports Snapshots to compact logs and speed up recovery.


### Volatile Store (`internal/store/volatile.go`):

Manages the in-memory state machine.

* Handles locking mechanisms to ensure isolation during the Prepare phase.
* Maintains the current value and the log of committed transaction IDs.


### Cluster Simulation (`main.go`):

* A simulation entry point that spins up 4 networked nodes (ports 3000-3003) within a single process.
* Demonstrates sequential transactions where different nodes take turns acting as the Coordinator.

## Key Features

* **Crash Recovery**: Nodes replay their WAL on startup to restore the last known consistent state. If a node crashes while `PREPARED`, it contacts the Coordinator to resolve the transaction status.
* **RPC Communication**: Uses Go's standard `net/rpc` for type-safe peer-to-peer communication.
* **Timeout Handling**: The Coordinator broadcasts aborts if peers fail to respond within a specific timeout window.
* **Concurrency Control**: Uses `sync.RWMutex` and distinct locking states to prevent race conditions during transaction processing.

## Project Structure

```
/
├── internal/
│   ├── node.go          # Core 2PC logic (Coordinator & Participant)
│   ├── node_rpc.go      # RPC handlers for network requests
│   ├── broadcast.go     # Helper for broadcasting messages to peers
│   ├── peer.go          # Client wrapper for dialing other nodes
│   ├── node_test.go     # Integration tests (Happy path, Abort, Recovery)
│   └── store/
│       ├── stable.go    # Disk persistence (WAL & Snapshots)
│       ├── volatile.go  # In-memory state & Locking
│       └── entry.go     # Log entry definitions
├── logs/                # Generated runtime logs (gitignored)
├── main.go              # Simulation entry point
├── Makefile             # Commands to run and test
└── go.mod

```

## How to Run

### Prerequisites

* Go (version 1.25 or later)
* `make`

### Running the Simulation

To see the protocol in action, run the main simulation. This starts a cluster of 4 nodes and executes a series of transactions.

```sh
make run

```

*Note: You may need to run `make clean` first if previous log files are causing conflicts, although the system is designed to recover.*

### Running Tests

The project includes comprehensive integration tests that verify success, failure, and recovery scenarios.

```sh
make test

```

Key tests included in `internal/node_test.go`:

* `TestTwoPhaseCommit_HappyPath`: Verifies a standard successful transaction.
* `TestTwoPhaseCommit_AbortOnPrepareFailure`: Simulates a node rejecting a proposal, causing a cluster-wide abort.
* `TestNodeRecovery_Persistence`: Writes data, crashes a node, restarts it, and verifies it recovers the correct state from disk.
* `TestCoordinator_Timeout`: Verifies that the coordinator aborts if a participant is unresponsive.
