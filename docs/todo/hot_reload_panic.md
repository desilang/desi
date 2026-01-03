# Hot-Reload and Process Supervision (Future)

This document describes future plans for Elixir-style hot-reload and how it affects error handling like division by zero.

## Current State

### Division by Zero
```desi
let x = 10 / 0  # Panics: exits program with code 1
```
- Prints error to stderr
- Exits immediately
- No recovery possible

## Future State (with Hot-Reload)

### Supervisor Trees
Like Elixir/Erlang, Desi will have supervisor processes that manage child processes:

```desi
# Future syntax (not implemented yet)
supervisor = Supervisor.new([
    Worker.spec(my_worker, restart: :permanent)
])

spawn supervisor.start()
```

### Division by Zero Evolution
When hot-reload is implemented:

1. **Panic becomes process-local**
   - Current: `panic` exits the whole program
   - Future: `panic` only terminates the current process
   
2. **Supervisors restart failed processes**
   - Parent supervisor notified of child crash
   - Supervisor restarts child based on strategy
   
3. **Message-passing for recovery**
   - Failed process drops its mailbox
   - Other processes can detect failures via monitors

### Implementation Notes

The current `__panic_divzero()` function in `builtins.c`:
```c
void __panic_divzero(void) {
    fprintf(stderr, "panic: integer division by zero\n");
    exit(1);
}
```

Will evolve to:
```c
// Future implementation
void __panic_divzero(void) {
    DesiProcess* current = __get_current_process();
    __process_set_exit_reason(current, "division by zero");
    __process_terminate(current);  // Signals supervisor, doesn't exit()
}
```

## Related Docs
- `docs/dev/runtime_panic.md` (to be created)
- `docs/dev/supervisor.md` (to be created)
