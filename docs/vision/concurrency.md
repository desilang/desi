# Desi Concurrency: A Vision for Fearless Parallelism

*Author: Desi Team*  
*Status: Design Specification*

---

## Introduction

When I set out to design Desi's concurrency model, I had one overarching goal: **make concurrent programming as safe and intuitive as sequential programming**. Too many developers avoid concurrency because existing solutions are either too complex (Rust's lifetimes across await points), too unsafe (C++ data races), or too limited (Python's GIL).

Desi takes the best ideas from Go, Rust, Swift, Kotlin, and Zig, combines them with our existing ownership model, and adds novel safety guarantees that no language has achieved before.

This document explains every design decision, the reasoning behind it, and how it all comes together to create what I believe is the most approachable yet powerful concurrency model in any systems language.

---

## Part 1: The Foundation - Why Desi Is Uniquely Positioned

### 1.1 Our Existing Safety Mechanisms

Before diving into concurrency, it's crucial to understand what Desi already provides:

```python
# Move semantics - values are moved, not copied by default
let data = create_large_buffer()
process(data)        # `data` moves into `process`
print(data)          # COMPILE ERROR: `data` has been moved

# Ownership tracking - single owner at any time
let owner = Resource.new()
let borrowed = owner.borrow()  # Temporary borrow
# owner still valid, borrowed expires at scope end

# RAII with `using` blocks
using file = File.open("data.txt"):
    file.write("Hello")
# File automatically closed here, even on error

# Arena allocation for controlled memory regions
using arena = Arena.new(1024):
    let buffer = arena.alloc[byte](256)
    # All arena allocations freed at block end

# Explicit error handling with Result/Option
def parse(input: str) -> Result[int, ParseError]:
    if input.is_empty():
        return Err(ParseError("empty input"))
    return Ok(int(input))
```

These foundations are **perfect** for building safe concurrency:

- **Move semantics** → Natural ownership transfer between tasks
- **No shared mutable state by default** → Eliminates 90% of data races
- **RAII** → Automatic cleanup of resources held by cancelled tasks
- **Result types** → Explicit error propagation across task boundaries

---

## Part 2: Runtime Design

### 2.1 The Lightweight M:N Runtime

Desi includes a lightweight runtime by default, inspired by Go's goroutine scheduler and Tokio's work-stealing implementation.

```
┌─────────────────────────────────────────────────────────────────┐
│                     Desi Runtime Architecture                    │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│   ┌─────────┐ ┌─────────┐ ┌─────────┐        M Tasks            │
│   │  Task   │ │  Task   │ │  Task   │           │               │
│   │   1     │ │   2     │ │   3     │           │               │
│   └────┬────┘ └────┬────┘ └────┬────┘           │               │
│        │           │           │                │               │
│        ▼           ▼           ▼                ▼               │
│   ┌─────────────────────────────────────────────────────────┐   │
│   │              Work-Stealing Scheduler                     │   │
│   │                                                          │   │
│   │   ┌──────────┐ ┌──────────┐ ┌──────────┐                │   │
│   │   │Worker 1  │ │Worker 2  │ │Worker N  │  N Workers     │   │
│   │   │ Queue    │ │ Queue    │ │ Queue    │                │   │
│   │   └────┬─────┘ └────┬─────┘ └────┬─────┘                │   │
│   └────────┼────────────┼────────────┼──────────────────────┘   │
│            │            │            │                          │
│            ▼            ▼            ▼                          │
│        ┌───────┐    ┌───────┐    ┌───────┐     OS Threads      │
│        │Thread │    │Thread │    │Thread │                      │
│        │   1   │    │   2   │    │  N    │                      │
│        └───────┘    └───────┘    └───────┘                      │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

**Why M:N Threading?**

| Approach | Pros | Cons |
|----------|------|------|
| 1:1 (One task = One thread) | Simple | Heavy (1MB+ stack each) |
| N:1 (Green threads, single core) | Lightweight | Can't use multiple cores |
| **M:N (Many tasks, Few threads)** | **Lightweight + Multi-core** | Complex runtime |

Go proved that M:N is the right default for most applications. Desi follows this approach with a runtime footprint of approximately **50-100KB**.

### 2.2 Zero-Runtime Option

For embedded systems, WebAssembly, or environments where you need full control, Desi provides a compile-time opt-out:

```python
#[no_runtime]
module embedded_app

# You provide the executor
def main():
    let executor = MyCustomExecutor.new()
    executor.run(my_main_task)
```

This compiles async functions to state machines without bundling any runtime, similar to Zig's approach.

---

## Part 3: Colorless Async - Solving the Function Coloring Problem

### 3.1 The Problem with Traditional Async

In most languages with async/await, you end up with two separate worlds:

```python
# Traditional approach (NOT Desi)
async def fetch_async(url: str) -> str:
    return await http.get(url)

def process_sync(data: str) -> str:
    return data.upper()

# Problem: Can't easily mix these!
def main():
    let data = fetch_async(url)  # ERROR: can't call async from sync
    process_sync(data)
```

This leads to "function coloring" - you must duplicate APIs or add `.sync()` wrappers everywhere.

### 3.2 Desi's Colorless Solution

In Desi, **the same code works in both sync and async contexts**:

```python
def fetch(url: str) -> str:
    return http.get(url)  # Blocking in sync context, awaited in async

def process(data: str) -> str:
    return data.upper()

# Sync context - blocking I/O
def main_sync():
    let data = fetch("https://api.example.com")  # Blocks thread
    print(process(data))

# Async context - non-blocking I/O
async def main_async():
    let data = fetch("https://api.example.com")  # Awaits automatically
    print(process(data))
```

**How It Works:**

1. Compiler analyzes call graph
2. Functions containing I/O are marked internally as "suspendable"
3. In async context: suspendable calls become await points
4. In sync context: suspendable calls block the thread
5. Same code, different execution strategy

**What Developers Have Been Asking For:**
> *"Why do I have to maintain two versions of every library - sync and async?"*

With Desi, you don't. One implementation works everywhere.

---

## Part 4: Safety Mechanisms - Compile-Time Guarantees

### 4.1 The Send and Sync Type Traits

Desi automatically determines whether types can safely cross thread boundaries:

```python
# Automatically derived as Send + Sync (all fields are Send + Sync)
struct Point:
    x: int
    y: int

# Automatically !Send (contains non-thread-safe type)
struct NotThreadSafe:
    connection: DatabaseConnection  # DB connections are !Send

# You can override when you know it's safe
@Send @Sync
struct ThreadSafeWrapper:
    inner: SomeExternalType  # You promise thread-safety
```

**The Safety Hierarchy:**

```
┌────────────────────────────────────────────────────────────────────┐
│                       Thread Safety Levels                          │
├────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  Level 0: Owned Values (Move)                                      │
│  ─────────────────────────────────────────────────────────────     │
│  • Can freely pass between tasks                                   │
│  • No synchronization needed                                       │
│  • Zero overhead                                                   │
│                                                                     │
│      let data = compute_result()                                   │
│      spawn:                                                        │
│          process(data)  # data moves into spawned task             │
│                                                                     │
├────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  Level 1: Shared Immutable (Arc<T>)                                │
│  ─────────────────────────────────────────────────────────────     │
│  • Multiple readers, no writers                                    │
│  • Reference counted                                               │
│  • Minimal overhead (atomic increment/decrement)                   │
│                                                                     │
│      let config = Arc.new(load_config())                           │
│      spawn:                                                        │
│          print(config.database_url)  # Safe read                   │
│      spawn:                                                        │
│          print(config.api_key)       # Safe read                   │
│                                                                     │
├────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  Level 2: Shared Mutable (Mutex<T> / RwLock<T>)                    │
│  ─────────────────────────────────────────────────────────────     │
│  • Synchronized access                                             │
│  • Compile-time checked                                            │
│  • Lock overhead                                                   │
│                                                                     │
│      let counter = Mutex.new(0)                                    │
│      spawn:                                                        │
│          counter.lock().increment()  # Exclusive access            │
│      spawn:                                                        │
│          counter.lock().increment()  # Waits for lock              │
│                                                                     │
├────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  Level 3: Actor Isolation                                          │
│  ─────────────────────────────────────────────────────────────     │
│  • No shared state at all                                          │
│  • Message passing only                                            │
│  • Maximum safety, some latency                                    │
│                                                                     │
│      actor Counter:                                                │
│          var count: int = 0                                        │
│          pub async def increment(self) -> int:                     │
│              self.count += 1                                       │
│              return self.count                                     │
│                                                                     │
└────────────────────────────────────────────────────────────────────┘
```

### 4.2 Automatic Capture Analysis

Unlike Swift's `@Sendable` annotations or Rust's explicit captures, Desi automatically analyzes what a closure captures and enforces safety:

```python
def example():
    let shared_data = Mutex.new([1, 2, 3])
    let config = Arc.new(Config.load())
    let local = 42
    
    spawn:
        # Compiler automatically:
        # 1. Sees `shared_data` captured - checks it's Sync ✓
        # 2. Sees `config` captured - clones the Arc ✓  
        # 3. Sees `local` captured - copies the int ✓
        
        let guard = shared_data.lock()
        guard.append(local)
        print(config.name)
    
    # No manual annotations needed!
```

### 4.3 Compile-Time Deadlock Detection (Novel Feature)

This is something **no mainstream language has implemented well**. Desi tracks lock acquisition order at compile time:

```python
let mutex_a = Mutex.new(1)
let mutex_b = Mutex.new(2)

spawn:
    let guard_a = mutex_a.lock()  # Lock order: A first
    let guard_b = mutex_b.lock()  # Then B
    # Order: A → B

spawn:
    let guard_b = mutex_b.lock()  # Lock order: B first
    let guard_a = mutex_a.lock()  # Then A
    # Order: B → A
    
# COMPILE ERROR:
# error[DCE0001] concurrency: potential deadlock detected
#   Task 1 acquires locks in order: mutex_a → mutex_b
#   Task 2 acquires locks in order: mutex_b → mutex_a
#   = help: acquire locks in consistent order across all tasks
```

**How It Works:**
1. Each `Mutex<T>` has a compile-time "lock ID"
2. Compiler tracks acquisition order in each code path
3. If any two paths acquire locks in different orders: error

### 4.4 Data Race Freedom Guarantee

Desi **guarantees at compile time** that data races cannot occur:

```python
let data = [1, 2, 3]

spawn:
    data.append(4)  # Captures `data` by move

spawn:
    print(data[0])  # COMPILE ERROR: `data` was moved to first spawn

# The fix is explicit and safe:
let data = Arc.new(Mutex.new([1, 2, 3]))

spawn:
    data.lock().append(4)  # Synchronized write

spawn:
    print(data.lock()[0])  # Synchronized read
```

---

## Part 5: Structured Concurrency

### 5.1 The Problem with Unstructured Spawning

Traditional `spawn` creates orphan tasks that outlive their parent:

```python
# Unstructured (other languages)
def problematic():
    spawn(background_task)  # Task lives forever!
    return  # Function returns but task keeps running
    
# What if background_task has an error?
# What if we need to cancel it?
# What if it holds resources we need to clean up?
```

### 5.2 Desi's TaskGroup - Structured by Default

Desi uses structured concurrency inspired by Swift and Kotlin:

```python
async def fetch_all_users(user_ids: list[int]) -> list[User]:
    async with TaskGroup() as group:
        for id in user_ids:
            group.spawn(fetch_user, id)
        
        # Implicit await: waits for ALL tasks to complete
        # If any task fails: other tasks are cancelled
        # If this scope exits early: all tasks cancelled
    
    return group.results()  # Collected results

# Benefits:
# 1. No orphan tasks - parent always waits
# 2. Automatic cancellation on error
# 3. Resource cleanup guaranteed
```

### 5.3 Cancellation Model

Cancellation is **cooperative and checked at await points**:

```python
async def download_large_file(url: str) -> bytes:
    let chunks = []
    
    async for chunk in http.stream(url):
        # ↑ Cancellation checked here at each await
        chunks.append(chunk)
        
        # Can also check manually:
        if Task.is_cancelled():
            # Cleanup before exiting
            cleanup_partial_download()
            raise CancellationError()
    
    return bytes.join(chunks)
```

---

## Part 6: Memory Model - Sequential Consistency by Default

### 6.1 The Problem with Relaxed Memory

Most languages expose CPU memory ordering directly, leading to subtle bugs:

```cpp
// C++: Easy to get wrong
std::atomic<int> x{0}, y{0};

// Thread 1:
x.store(1, std::memory_order_relaxed);
y.store(1, std::memory_order_release);  # Which ordering again?

// Thread 2:
if (y.load(std::memory_order_acquire)) {  # Do I need acquire here?
    assert(x.load(std::memory_order_relaxed) == 1);  # Is this safe?
}
```

### 6.2 Desi's Approach: Safe Default, Unsafe Opt-In

```python
# Default: Sequential consistency (safe, predictable)
let counter = Atomic.new(0)

spawn:
    counter.store(42)  # SeqCst by default

spawn:
    let value = counter.load()  # Guaranteed to see 42 if store happened
    print(value)

# Opt-in relaxed for performance-critical code (rare)
let fast_counter = Atomic.new(0)
fast_counter.store(42, ordering=Relaxed)  # You explicitly requested unsafe
```

**Rationale:** 99% of developers don't need to think about memory ordering. The 1% who do can opt into relaxed semantics explicitly.

---

## Part 7: Error Propagation Across Tasks

### 7.1 The Problem

What happens when a spawned task fails?

```python
# Other languages: Various bad behaviors
spawn:
    raise SomeError("oops")  # Silent failure? Crash? Log somewhere?
```

### 7.2 Desi's Solution: Result-Based Aggregation

```python
async def fetch_many(urls: list[str]) -> Result[list[str], list[Error]]:
    async with TaskGroup() as group:
        for url in urls:
            group.spawn(fetch, url)
    
    # Option 1: Fail-fast (first error cancels siblings)
    match group.try_results():
        case Ok(results):
            return Ok(results)
        case Err(error):
            return Err([error])  # First error, others cancelled
    
    # Option 2: Collect all (wait for all, aggregate errors)
    match group.results_all():
        case Ok(results):
            return Ok(results)
        case Err(errors):
            return Err(errors)  # All errors from all failed tasks
```

---

## Part 8: Channels - Safe Message Passing

### 8.1 Basic Channel Usage

```python
# Create a bounded channel (backpressure when full)
let (tx, rx) = Channel[int].new(buffer=10)

# Producer task
spawn:
    for i in range(100):
        tx.send(i)      # Blocks if buffer full
    tx.close()          # Signal completion

# Consumer task  
spawn:
    for item in rx:     # Iterates until channel closed
        print(item)
```

### 8.2 Select for Multiplexing

```python
let (tx1, rx1) = Channel[str].new()
let (tx2, rx2) = Channel[int].new()

async def multiplexer():
    loop:
        select:
            case msg = rx1.recv():
                print(f"String: {msg}")
            
            case num = rx2.recv():
                print(f"Number: {num}")
            
            case after(5.seconds):
                print("Timeout - no messages for 5 seconds")
                break
```

### 8.3 Channel Ownership

Channels enforce ownership - you can't accidentally share the wrong end:

```python
let (tx, rx) = Channel[int].new()

# Clone sender for multiple producers
let tx2 = tx.clone()

spawn:
    tx.send(1)   # Producer 1

spawn:
    tx2.send(2)  # Producer 2

# Receiver cannot be cloned - single consumer
spawn:
    for item in rx:
        print(item)
```

---

## Part 9: Actors - Maximum Isolation

### 9.1 When to Use Actors

Actors are ideal when:
- You want **zero shared state**
- You need **location transparency** (local or remote)
- You want **crash isolation** (one actor failing doesn't kill others)

### 9.2 Actor Syntax

```python
actor BankAccount:
    var balance: decimal = 0.0
    var transaction_log: list[str] = []
    
    pub async def deposit(self, amount: decimal) -> Result[decimal, Error]:
        if amount <= 0:
            return Err(Error("Amount must be positive"))
        
        self.balance += amount
        self.transaction_log.append(f"Deposit: {amount}")
        return Ok(self.balance)
    
    pub async def withdraw(self, amount: decimal) -> Result[decimal, Error]:
        if amount > self.balance:
            return Err(Error("Insufficient funds"))
        
        self.balance -= amount
        self.transaction_log.append(f"Withdraw: {amount}")
        return Ok(self.balance)
    
    pub async def get_balance(self) -> decimal:
        return self.balance

# Usage
async def main():
    let account = BankAccount.spawn()
    
    await account.deposit(100.0)?
    await account.withdraw(30.0)?
    
    let balance = await account.get_balance()
    print(f"Balance: {balance}")  # 70.0
```

### 9.3 Actor Guarantees

1. **Single-threaded execution** - Only one message processed at a time
2. **No shared state** - Actor owns all its data
3. **Async interface** - All methods are implicitly async
4. **Crash isolation** - Actor crash doesn't affect others

---

## Part 10: Complete Example - A Concurrent Web Scraper

Here's a comprehensive example showing all features working together:

```python
import http
import html

# Configuration loaded once, shared immutably
struct Config:
    max_concurrent: int
    timeout_seconds: int
    user_agent: str

# Result of scraping a page
struct PageResult:
    url: str
    title: str
    links: list[str]

# Actor for rate limiting
actor RateLimiter:
    var requests_this_second: int = 0
    var last_reset: Timestamp = Timestamp.now()
    
    pub async def acquire(self) -> bool:
        let now = Timestamp.now()
        if now - self.last_reset > 1.seconds:
            self.requests_this_second = 0
            self.last_reset = now
        
        if self.requests_this_second >= 10:  # Max 10 req/sec
            return false
        
        self.requests_this_second += 1
        return true

# Main scraping function
async def scrape_page(url: str, config: Arc[Config]) -> Result[PageResult, Error]:
    let response = await http.get(url, 
        timeout=config.timeout_seconds.seconds,
        headers={"User-Agent": config.user_agent}
    )?
    
    let doc = html.parse(response.body)
    let title = doc.select_one("title")?.text() ?? "Untitled"
    let links = [a.attr("href") for a in doc.select("a[href]")]
    
    return Ok(PageResult(url=url, title=title, links=links))

# Concurrent scraper with rate limiting
async def scrape_all(urls: list[str], config: Config) -> list[Result[PageResult, Error]]:
    let shared_config = Arc.new(config)
    let rate_limiter = RateLimiter.spawn()
    
    # Bounded concurrency with semaphore
    let semaphore = Semaphore.new(config.max_concurrent)
    
    async with TaskGroup() as group:
        for url in urls:
            group.spawn(async def():
                # Wait for rate limit
                while not await rate_limiter.acquire():
                    await sleep(100.milliseconds)
                
                # Acquire semaphore slot
                async with semaphore.acquire():
                    return await scrape_page(url, shared_config)
            )
    
    return group.results_all()

# Main entry point
def main() -> int:
    let config = Config(
        max_concurrent=5,
        timeout_seconds=30,
        user_agent="Desi Scraper/1.0"
    )
    
    let urls = [
        "https://example.com",
        "https://example.org",
        "https://example.net",
        # ... more URLs
    ]
    
    let results = scrape_all(urls, config)
    
    for result in results:
        match result:
            case Ok(page):
                print(f"Scraped: {page.title} ({len(page.links)} links)")
            case Err(error):
                print(f"Failed: {error}")
    
    return 0
```

---

## Part 11: How Desi Stands Out

### 11.1 Comparison with Existing Languages

| Feature | Go | Rust | Swift | Kotlin | **Desi** |
|---------|-----|------|-------|--------|----------|
| Function Coloring | No async | Colored | Colored | Colored | **Colorless** |
| Compile-Time Race Detection | No | Yes | Partial | No | **Yes** |
| Compile-Time Deadlock Detection | No | No | No | No | **Yes** |
| Structured Concurrency | No | No | Yes | Yes | **Yes** |
| Automatic Send/Sync | N/A | Manual | Manual | N/A | **Automatic** |
| Zero-Runtime Option | No | Yes | No | No | **Yes** |
| Python-Like Syntax | No | No | No | Close | **Yes** |

### 11.2 Our Unique Contributions

1. **Colorless Async** - Same code works sync or async
2. **Compile-Time Deadlock Detection** - First systems language to achieve this
3. **Automatic Capture Analysis** - No `@Sendable` annotations
4. **Python Ergonomics + Rust Safety** - The best of both worlds
5. **Borrow Across Await** - Compiler handles the complexity

### 11.3 The Desi Promise

When you write concurrent code in Desi, you get:

✅ **No data races** - Guaranteed at compile time  
✅ **No deadlocks** - Detected at compile time  
✅ **No orphan tasks** - Structured concurrency  
✅ **No function coloring** - One codebase for sync and async  
✅ **No manual annotations** - Compiler figures it out  
✅ **No runtime if you don't want it** - Embedded-friendly  

---

## Conclusion

Desi's concurrency model represents a significant step forward in making parallel programming accessible and safe. By building on our existing ownership model and learning from every major language, we've created something that is:

- **Safer than Go** (compile-time race detection)
- **Simpler than Rust** (no lifetime annotations across await)
- **More flexible than Swift** (colorless async)
- **More powerful than Python** (true parallelism)

We invite the community to explore, contribute, and help us refine this vision. Together, we can make concurrent programming fearless for everyone.

---

*For implementation details, see [Iterator Protocol Implementation](iterator-protocol.md) as a reference for our approach to compiler features.*

*For contribution guidelines, see [CONTRIBUTING.md](../CONTRIBUTING.md).*
