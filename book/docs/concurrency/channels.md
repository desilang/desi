# Channels - Message Passing Between Tasks

**Channels** provide safe communication between concurrent tasks. Instead of sharing memory directly (like Mutex), channels let tasks send messages to each other.

## Creating a Channel

Use `channel_new(capacity)` to create a buffered channel:

```desi
# Create a channel with buffer size 10
let ch = channel_new(10)
```

The `capacity` determines how many messages can be buffered before `send` blocks.

## Sender and Receiver

Channels split into two handles:
- **Sender**: For sending values into the channel
- **Receiver**: For receiving values from the channel

```desi
let ch = channel_new(10)

# Get the sender and receiver handles
let sender = ch.sender()
let receiver = ch.receiver()
```

### Why Split?

This enables:
1. **Ownership tracking**: Know who can send vs receive
2. **Multiple producers**: Clone senders for many-to-one patterns
3. **Single consumer**: Typically one receiver (MPSC pattern)

## Sending Values

Use `send(value)` to send a value:

```desi
let sender = ch.sender()

# Send returns bool: true if successful, false if channel closed
let ok = sender.send(42)
print(ok)  # true
```

### Blocking Behavior

- `send()` **blocks** if the buffer is full
- `try_send()` returns immediately (non-blocking)

```desi
# Non-blocking send
let ok = sender.try_send(value)
if not ok:
    print("Channel full or closed")
```

## Receiving Values

Use `recv()` to receive a value:

```desi
let receiver = ch.receiver()

# recv returns Option[T]: Some(value) or Nothing if closed
match receiver.recv():
    Option.Some(value):
        print("Received: " + str(value))
    Option.Nothing:
        print("Channel closed")
```

### Blocking Behavior

- `recv()` **blocks** if the buffer is empty
- `try_recv()` returns immediately (non-blocking)

```desi
# Non-blocking receive
match receiver.try_recv():
    Option.Some(value):
        process(value)
    Option.Nothing:
        # Nothing available right now
        pass
```

## Closing a Channel

Close a channel to signal no more values will be sent:

```desi
ch.close()
```

After closing:
- `send()` returns `false`
- `recv()` returns `Nothing` once the buffer is empty

## Complete Example

```desi
def main() -> int:
    # Create a channel
    let ch = channel_new(10)
    let sender = ch.sender()
    let receiver = ch.receiver()
    
    # Send some values
    sender.send(1)
    sender.send(2)
    sender.send(3)
    
    # Receive and print
    let v1 = receiver.try_recv()  # Some(1)
    let v2 = receiver.try_recv()  # Some(2)
    let v3 = receiver.try_recv()  # Some(3)
    let v4 = receiver.try_recv()  # Nothing (empty)
    
    # Close the channel
    ch.close()
    
    return 0
```

## Channel Patterns

### Producer-Consumer

```desi
# Producer task
async def producer(sender: Sender):
    for i in range(100):
        sender.send(i)
    # Sender dropped when function returns

# Consumer task
async def consumer(receiver: Receiver):
    loop:
        match receiver.recv():
            Option.Some(value):
                process(value)
            Option.Nothing:
                break  # Channel closed
```

### Work Queue

```desi
# Multiple workers can share cloned senders
let ch = channel_new(100)
let sender1 = ch.sender()
let sender2 = sender1.clone()  # Multiple producers

# Single consumer processes all work
let receiver = ch.receiver()
```

## When to Use Channels

Use channels when:
- Tasks need to communicate results
- You want to decouple producers from consumers
- Order of messages matters (FIFO)

Consider alternatives:
- **Mutex**: For shared state that multiple tasks read/write
- **TaskGroup**: For structured concurrency (waiting for subtasks)

## Comparison: Channel vs Mutex

| Feature | Channel | Mutex |
|---------|---------|-------|
| Pattern | Message passing | Shared state |
| Blocking | Yes (when full/empty) | Yes (when locked) |
| Data flow | One-way | In-place |
| Best for | Pipelines, queues | Counters, caches |

## See Also

- [Mutex](./mutex.md) - Thread-safe shared state
- [Concurrency Overview](./thread-safety.md) - Thread safety and tasks
