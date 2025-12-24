# Select - Multiplexing Channel Operations

The `select` statement waits on multiple channel operations and executes the first one that becomes ready.

## Basic Syntax

```desi
select:
    case msg = rx1.try_recv():
        print("Got from ch1: " + str(msg))
    case rx2.try_recv():
        print("Got from ch2")
    default:
        print("Nothing ready")
```

## Cases

### Receive with Binding
```desi
case msg = receiver.try_recv():
    # msg contains the received value
    process(msg)
```

### Receive without Binding
```desi
case receiver.try_recv():
    # Just react to availability
    print("Got something")
```

### Send
```desi
case sender.try_send(value):
    print("Sent successfully")
```

### Default
```desi
default:
    # Runs if no other case is ready
    print("Nothing ready")
```

## Example: Timeout Pattern

```desi
let ch = channel_new(10)
let rx = ch.receiver()

select:
    case msg = rx.try_recv():
        print("Got: " + str(msg))
    default:
        print("Timed out")
```

## When to Use Select

Use select when:
- You need to wait on multiple channels
- You want non-blocking communication attempts
- Building event-driven patterns

## See Also

- [Channels](./channels.md) - Basic channel operations
- [Mutex](./mutex.md) - Shared state protection
- [TaskGroup](./taskgroup.md) - Structured concurrency
