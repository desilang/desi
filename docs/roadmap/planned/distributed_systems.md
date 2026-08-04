# Distributed Systems — Design Vision (v0.2.0+)

> **Status**: Vision/research phase. Not planned for v0.1.0.
>
> **Inspiration**: Erlang/OTP Distributed Erlang, Elixir `Node` module.

---

## Why This Matters

Erlang/Elixir's killer feature is **distributed processes as a runtime concept**. Most languages give you networking libraries; Erlang gives you location-transparent message passing between nodes.

Desi's concurrency model (channels, TaskGroups, supervisors) already borrows from Erlang/Go. Extending this to **cross-node communication** would complete the picture.

---

## The Challenge for Desi

Desi compiles to **native code via LLVM**. There is no VM. This means:

| Erlang/BEAM feature | Desi equivalent | Feasibility |
|---|---|---|
| Lightweight processes (millions) | OS threads + TaskGroup | ⚠️ Heavier, but workable |
| Location-transparent messaging | Would need serialized TCP messages | ✅ Possible |
| Node discovery (EPMD) | Custom service discovery | ✅ Possible |
| Hot code loading per-process | Module-level dlopen/dlclose | ⚠️ Coarser granularity |
| Process linking/monitoring across nodes | Distributed supervisor protocol | ✅ Possible |

**Key insight**: We can't replicate BEAM's lightweight processes, but we CAN build a **distributed actor/messaging layer** on top of Desi's existing concurrency primitives.

---

## Proposed Design

### Node Connection

```desi
import distributed

# Start this process as a named node
distributed.start_node("app1@192.168.1.10")

# Connect to another node
distributed.connect("app2@192.168.1.20")

# Check connected nodes
let nodes = distributed.list_nodes()
```

### Cross-Node Messaging

```desi
# Send a message to a named process on another node
distributed.send("worker@app2", {"task": "process", "data": payload})

# Receive messages (in a GenServer-like loop)
def handle_message(msg: dict[str, Any]) -> none:
    match msg["task"]:
        "process": do_work(msg["data"])
        "status": reply_status()
```

### Distributed Supervisor

```desi
import distributed

# Start a supervisor that manages workers across nodes
using sup = distributed.Supervisor():
    sup.start_child("worker_1", worker_fn, node="app2@host2")
    sup.start_child("worker_2", worker_fn, node="app3@host3")
    # Workers auto-restart on crash, even on remote nodes
```

---

## Protocol Design

### Wire Protocol
- **Serialization**: Binary format (not JSON — too slow for high-frequency messaging)
- **Transport**: TCP with optional TLS
- **Authentication**: Shared secret cookie (like Erlang's magic cookie)
- **Heartbeat**: Periodic pings to detect node failures

### Service Discovery
- **Option A**: Central registry (like EPMD) — simpler, single point of failure
- **Option B**: Gossip protocol — decentralized, more resilient
- **Option C**: DNS-based discovery — works with existing infrastructure

### Message Guarantees
- **At-most-once** delivery by default (like Erlang)
- **At-least-once** via optional ack/retry protocol
- **Ordering**: Per-sender FIFO (like Erlang)

---

## What This Would NOT Be

- **Not a replacement for gRPC/HTTP** for public APIs
- **Not a distributed database** — still need Postgres/Redis for persistence
- **Not magic** — network failures, split-brain, and consistency are still hard
- **Not a VM** — Desi remains compiled, so no process-level hot code swapping

---

## Implementation Phases

### Phase 1: Node Connection + Simple Messaging
- `distributed.start_node()` / `distributed.connect()`
- `distributed.send()` / `distributed.recv()`
- Binary serialization for basic types (int, str, list, dict)

### Phase 2: Distributed Supervisor
- Remote `start_child()` with node targeting
- Cross-node crash detection and restart
- Health monitoring and heartbeats

### Phase 3: Advanced Features
- Process groups (pub/sub across nodes)
- Load balancing and work distribution
- Node discovery protocols

---

## Dependencies

This feature requires:
1. ✅ `sync` module (channels, supervisors) — already done
2. ✅ `net` module (TCP sockets) — already done
3. ✅ Serialization capabilities — json exists, binary format needed
4. `[ ]` Binary serialization format design
5. `[ ]` Node protocol specification
6. `[ ]` Service discovery mechanism

---

## Research References

- [Erlang Distribution Protocol](https://www.erlang.org/doc/apps/erts/erl_dist_protocol.html)
- [Elixir Node module](https://hexdocs.pm/elixir/Node.html)
- [Phoenix.PubSub](https://hexdocs.pm/phoenix_pubsub/Phoenix.PubSub.html)
- [Akka Actor Model](https://doc.akka.io/docs/akka/current/typed/index.html)
- [Orleans Virtual Actors](https://learn.microsoft.com/en-us/dotnet/orleans/)
