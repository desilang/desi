# redis — Redis Client

Pure RESP protocol Redis client. No external dependencies.

## Import

```desi
import redis
```

## Connection

```desi
redis.connect("127.0.0.1", 6379)
let pong = redis.ping()   # "PONG"
redis.close()
```

## String Commands

```desi
redis.set_val("user:1:name", "Alice")
let name = redis.get_val("user:1:name")  # "Alice"

redis.set_val("counter", "10")
redis.incr("counter")  # 11
redis.decr("counter")  # 10

redis.del_key("user:1:name")
let e = redis.exists("user:1:name")  # 0
```

## Expiry (TTL)

```desi
redis.setex("session:abc", 3600, "user_data")  # expires in 1 hour
let remaining = redis.ttl("session:abc")        # seconds left
redis.expire("key", 60)                         # set TTL on existing key
```

## Hash Commands

Store objects as field-value pairs:

```desi
redis.hset("user:1", "name", "Alice")
redis.hset("user:1", "age", "30")
let name = redis.hget("user:1", "name")  # "Alice"
redis.hdel("user:1", "age")
```

## List Commands

Queues and stacks:

```desi
redis.rpush("queue", "task1")
redis.rpush("queue", "task2")
redis.rpush("queue", "task3")
let length = redis.llen("queue")    # 3
let first = redis.lpop("queue")     # "task1" (FIFO)
let last = redis.rpop("queue")      # "task3" (LIFO)
```

## API Reference

| Function | Description |
|---|---|
| `connect(host, port)` | Connect to Redis server |
| `close()` | Close connection |
| `ping()` | Returns "PONG" |
| `set_val(key, value)` | SET string value |
| `get_val(key)` | GET string value |
| `del_key(key)` | Delete key |
| `exists(key)` | Check if key exists (1/0) |
| `setex(key, secs, val)` | SET with expiry |
| `expire(key, secs)` | Set TTL on key |
| `ttl(key)` | Get remaining TTL |
| `incr(key)` | Increment by 1 |
| `decr(key)` | Decrement by 1 |
| `hset(key, field, val)` | Set hash field |
| `hget(key, field)` | Get hash field |
| `hdel(key, field)` | Delete hash field |
| `lpush(key, val)` | Push to list head |
| `rpush(key, val)` | Push to list tail |
| `lpop(key)` | Pop from list head |
| `rpop(key)` | Pop from list tail |
| `llen(key)` | Get list length |
| `flushdb()` | Delete all keys |
