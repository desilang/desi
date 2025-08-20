# Concurrency (Stage-0 subset)

Desi is **message-passing first**. Stage-0 provides channels and `spawn` for lightweight tasks. Actors and supervisors are library-level atop channels.

## Goals (Stage-0)
- Safe concurrency defaults (no shared mutable state by accident).
- Deterministic primitives that lower cleanly to C (pthread/Win32).
- A path to async I/O and schedulers in later stages.

## Primitives

### 1) `spawn`
Start a new task running a function. Returns a task handle `Task` (opaque in Stage-0).

```desi
def worker(n: i32) -> void:
  io.println(fmt.int(n))

def main() -> i32:
  let t = spawn worker(42)
  0
````

Lowering (C): create OS thread or submit to a fixed thread pool.

### 2) Channels `chan[T]`

Typed FIFO queues for communication.

APIs (minimum):

* `chan[T].bounded(cap: u32) -> Chan[T]`
* `send(ch: Chan[T], v: T) -> bool`          # false on closed
* `recv(ch: Chan[T]) -> Option[T]`           # None on closed/timeout
* `close(ch: Chan[T]) -> void`

```desi
def main() -> i32:
  let ch = chan[i32].bounded(16)
  let _ = spawn def() -> void:
    send(ch, 123)
  match recv(ch):
    Some(v) => io.println(fmt.int(v))
    None    => io.println("closed")
  0
```

### 3) Timeouts (optional arg)

`recv(ch, timeout: Duration = inf) -> Option[T]`

```desi
match recv(ch, 1s):
  Some(v) => handle(v)
  None    => io.println("timeout")
```

### 4) Select (Stage-0 optional; can be added in Stage-1)

A simple `select` can be provided as a library helper; native syntax may come later.

## Ownership across tasks

* Sending moves ownership of the payload into the channel.
* ARC makes `str`/`Vec[T]` safe to transfer; refcounts are atomic.

## Actor library (layered on channels)

* `Pid[T]` wraps a `Chan[T]`.
* `self() -> Pid[T]` (generic erased in Stage-0, type checked at call sites).
* `send(pid, msg)` is sugar over `send(pid.mailbox, msg)`.

```desi
def ponger(me: Pid[str]) -> void:
  loop:
    match recv(me.inbox):
      Some("ping") => send(me.reply_to, "pong")
      _            => break
```

## Fault tolerance

* Stage-0: provide a simple `Supervisor` library with `one_for_one` restart strategy.
* Crashes are process-aborting by default unless wrapped in a supervised task launcher.

## Memory model

* Channels retain payloads while queued and release when taken.
* No shared mutable state unless user deliberately shares a `mut` owner through a single channel; sharing the same owner through multiple channels is discouraged and may be linted.

## Implementation notes (C backend)

* Unix: pthreads + condition variables; Windows: Win32 threads + events.
* Timeouts: condvar timed waits or OS wait APIs.
* Bounded channel: MPSC ring buffer with a mutex/condvar (Stage-0); lock-free later.
