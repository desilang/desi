# Channels in Desi

Channels provide thread-safe message passing between concurrent tasks. They're the primary way to communicate data between `spawn`ed tasks.

## Quick Start

```desi
import sync

def main() -> int:
    # Create a bounded channel with capacity 10
    let ch = sync.Channel(10)
    
    # Get sender and receiver handles
    let tx = ch.sender()
    let rx = ch.receiver()
    
    # Send a value
    tx.send(42)
    
    # Receive the value
    match rx.recv():
        Option.Some(v): print(v)  # Prints: 42
        Option.Nothing: print("Channel closed")
    
    return 0
```

## Creating Channels

Channels are bounded - they have a fixed capacity:

```desi
import sync

# Create channel with capacity 10
let ch = sync.Channel(10)

# Or use from-import
from sync import Channel
let ch = Channel(10)
```

## Senders and Receivers

Channels use separate sender and receiver handles:

```desi
let tx = ch.sender()    # Get a sender
let rx = ch.receiver()  # Get a receiver

# Send values
tx.send(1)
tx.send(2)
tx.send(3)

# Close when done sending
ch.close()
```

## Sending Values

### Blocking Send

`send()` blocks if the channel is full:

```desi
tx.send(42)  # Blocks if channel is at capacity
```

### Non-blocking Send

`try_send()` returns immediately:

```desi
if tx.try_send(42):
    print("Sent!")
else:
    print("Channel full or closed")
```

## Receiving Values

### Blocking Receive

`recv()` blocks until a value is available:

```desi
match rx.recv():
    Option.Some(value): print(value)
    Option.Nothing: print("Channel closed")
```

### Non-blocking Receive

`try_recv()` returns immediately:

```desi
match rx.try_recv():
    Option.Some(value): print(value)
    Option.Nothing: print("No value available")
```

## Closing Channels

Close a channel to signal no more values will be sent:

```desi
ch.close()
```

After closing:
- `send()` and `try_send()` return `false`
- `recv()` returns remaining values, then `Nothing`

## Example: Producer-Consumer

```desi
import sync

def main() -> int:
    let ch = sync.Channel(5)
    let tx = ch.sender()
    let rx = ch.receiver()
    
    # Send multiple values
    tx.send(1)
    tx.send(2)
    tx.send(3)
    
    # Close channel
    ch.close()
    
    print("Producer-Consumer complete!")
    return 0
```

## Best Practices

1. **Always close channels** when done sending to prevent receiver blocking
2. **Use bounded channels** to prevent unbounded memory growth
3. **Use `try_send`/`try_recv`** when you don't want to block
4. **Handle `Nothing`** case when receiving - channel may be closed

## Import Styles

Both import styles work:

```desi
# Module import
import sync
let ch = sync.Channel(10)

# Direct import  
from sync import Channel
let ch = Channel(10)
```

## See Also

- [Mutex](./mutex.md) - For protecting shared data
- [TaskGroup](./taskgroup.md) - For creating concurrent tasks
