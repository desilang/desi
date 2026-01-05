# Channel Implementation

This document explains the implementation of `Channel[T]`, `Sender[T]`, and `Receiver[T]` in the Desi compiler.

## Architecture Overview

Channels provide message passing between concurrent tasks:

```
Producer Task                                      Consumer Task
─────────────                                      ─────────────
let ch = channel_new(10)                           
let sender = ch.sender()      ────►  Channel  ────►  let receiver = ch.receiver()
sender.send(value)            ────►  Buffer   ────►  receiver.recv()
```

## Type Definitions

### `compiler/internal/types/channel.go`

```go
type Channel struct {
    Elem T // The element type being sent/received
}

type ChannelSender struct {
    Elem T // The element type
}

type ChannelReceiver struct {
    Elem T // The element type
}
```

Key design decisions:
- **Channel splits into Sender + Receiver**: Enables ownership tracking
- **MPSC pattern**: Multiple senders, single receiver (typical pattern)

## Type Checker Integration

### 1. `channel_new()` Builtin

**File**: `check/info.go`
```go
addN("channel_new",
    []types.T{types.Int}, // Buffer capacity
    []ast.ParamMode{ast.ParamMove},
    nil, // Channel[T]
    []string{"capacity"},
)
```

**File**: `check/expr_call.go`
```go
if id.Name == "channel_new" && len(args) == 1 {
    // Creates Channel[Any] - for typed channels, use generics (future)
    channelType := types.ChannelOf(types.Any)
    c.info.Types[call] = channelType
    return channelType
}
```

### 2. Channel Methods

**File**: `check/expr_field.go`
```go
func (c *checker) resolveChannelMethod(x *ast.FieldExpr, ch *types.Channel) types.T {
    switch name {
    case "sender":
        return types.FuncOf(nil, types.SenderOf(ch.Elem), false)
    case "receiver":
        return types.FuncOf(nil, types.ReceiverOf(ch.Elem), false)
    case "close":
        return types.FuncOf(nil, types.None, false)
    }
}
```

### 3. Sender Methods

```go
func (c *checker) resolveChannelSenderMethod(x *ast.FieldExpr, s *types.ChannelSender) types.T {
    switch name {
    case "send":
        return types.FuncOf([]types.T{s.Elem}, types.Bool, false)
    case "try_send":
        return types.FuncOf([]types.T{s.Elem}, types.Bool, false)
    }
}
```

### 4. Receiver Methods

```go
func (c *checker) resolveChannelReceiverMethod(x *ast.FieldExpr, r *types.ChannelReceiver) types.T {
    switch name {
    case "recv":
        return types.FuncOf(nil, types.OptionOf(r.Elem), false)
    case "try_recv":
        return types.FuncOf(nil, types.OptionOf(r.Elem), false)
    }
}
```

## Lowering to LLVM IR

### channel_new

**File**: `lower/lower_call.go`
```go
case "channel_new":
    res := ls.b.FreshTemp("channel")
    cap64 := ls.b.FreshTemp("cap64")
    ls.b.Emit(&hir.Cast{Src: argVal, Dst: cap64, Type: "i64"})
    ls.b.Emit(&hir.Call{Dst: res, Fn: "channel_new", Args: []hir.Value{cap64}, Type: "ptr"})
    return res
```

### sender.send(value)

```go
// Box the value to ptr
boxPtr := ls.b.FreshTemp("send_box")
ls.b.Emit(&hir.Call{Dst: boxPtr, Fn: "malloc", Args: []hir.Value{hir.ConstInt{Text: "8"}}, Type: "ptr"})
ls.b.Emit(&hir.Store{Dst: boxPtr, Val: argVal})
// Send the boxed value
ls.b.Emit(&hir.Call{Dst: dst, Fn: "channel_send", Args: []hir.Value{senderVal, boxPtr}, Type: "i1"})
```

### receiver.recv()

```go
// Returns ptr (boxed value or null)
ls.b.Emit(&hir.Call{Dst: dst, Fn: "channel_recv", Args: []hir.Value{receiverVal}, Type: "ptr"})
```

## C Runtime

### Platform Abstraction

**File**: `runtime/channel.h`
```c
typedef struct {
    void** buffer;              
    int64_t capacity;           
    int64_t head, tail, count;  
    
    DesiPlatformMutex lock;     // pthread_mutex_t or SRWLOCK
    DesiPlatformCond not_full;  // pthread_cond_t or CONDITION_VARIABLE
    DesiPlatformCond not_empty; 
    
    bool closed;                
    int senders, receivers;     
} DesiChannel;
```

### Key Functions

| Function | Description |
|----------|-------------|
| `channel_new(capacity)` | Create bounded channel |
| `channel_sender(ch)` | Create sender handle |
| `channel_receiver(ch)` | Create receiver handle |
| `channel_send(s, value)` | Send value (blocks if full) |
| `channel_try_send(s, value)` | Non-blocking send |
| `channel_recv(r)` | Receive value (blocks if empty) |
| `channel_try_recv(r)` | Non-blocking receive |
| `channel_close(ch)` | Close channel |

### Blocking Behavior

Using platform macros from `platform.h`:

```c
bool channel_send(ChannelSender* s, void* value) {
    DESI_MUTEX_LOCK(ch->lock);
    
    // Wait while buffer is full
    while (ch->count >= ch->capacity && !ch->closed) {
        DESI_COND_WAIT(ch->not_full, ch->lock);
    }
    
    // Add to circular buffer
    ch->buffer[ch->tail] = value;
    ch->tail = (ch->tail + 1) % ch->capacity;
    ch->count++;
    
    DESI_COND_SIGNAL(ch->not_empty);
    DESI_MUTEX_UNLOCK(ch->lock);
    return true;
}
```

## Known Limitations

1. **Channel[Any]**: Currently creates untyped channels; proper generic syntax needed
2. **No unbounded channels**: Buffer capacity required (uses linked list stub)
3. **Return type**: `recv()` returns `Option[Any]`, not `Option[T]`

## Related Files

- `compiler/internal/types/channel.go` - Type definitions
- `compiler/internal/check/expr_field.go` - Method resolution
- `compiler/internal/check/expr_call.go` - `channel_new` builtin
- `compiler/internal/lower/lower_call.go` - Lowering
- `compiler/runtime/channel.c` - C implementation
- `compiler/runtime/channel.h` - C headers
- `compiler/runtime/platform.h` - Platform abstraction
