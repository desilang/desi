# Windows Build Guide

## Prerequisites

- **Go 1.21+** — [go.dev/dl](https://go.dev/dl/)
- **LLVM / Clang 15+** — [llvm.org/releases](https://releases.llvm.org/) (installer adds `clang` to PATH)
- **Visual Studio Build Tools 2022** with the *Desktop development with C++* workload — provides `cl.exe`, `lib.exe`, `link.exe`

## Build

```powershell
# Full clean build (runtime + all Go binaries)
.\build.ps1 -Clean
.\build.ps1

# Output
bin\desic.exe
bin\desifmt.exe
bin\desirepl.exe
bin\desilsp.exe
build\libdesi.lib
```

`build.ps1` is the Windows equivalent of `make clean && make`.

## How the Build Works

**Runtime library (`build\libdesi.lib`):**
MSVC `cl.exe` compiles each `.c` file in `compiler/runtime/` to `.obj`, then `lib.exe` merges them with `libmpdec.lib` into `libdesi.lib`.

Unix-only files are excluded (they use APIs unavailable on Windows):
```
desi_host.c   signal_handler.c
```
Everything else builds on Windows, including the HTTP server, WebSocket, and TLS
modules which were ported to WinSock2 / Win32 threading / Schannel.

Key ports: `os.c`/`net.c` (WinSock2), `fs.c`/`path.c` (dirent shim, `_fullpath`),
`random.c`/`uuid.c` (`rand_s`), `process.c` (CreateProcess), `shell.c` (`_popen`,
FindFirstFile glob), `re.c` (bundled minimal-ERE regex shim), `http_server.c`
(WinSock2 + platform threads), `websocket.c` (closesocket, TLS-qual macros),
`tls.c` (Schannel via `tls_win.h`), `reload.c` (portable C).

The `compiler/runtime/db/` directory (`RUNTIME_DB` in the Makefile) is also excluded: `pool.c` uses raw pthreads and `mysql.c`/`redis.c`/`db_timeout.h` use POSIX sockets. The db/ORM modules need a Win32 port before the `db` examples (437–446) can work on Windows.

**Go binaries:** Standard `go build` — no platform differences.

**User programs (`desic run/build`):**
1. Go compiler lowers Desi → LLVM IR
2. `clang -c program.ll -o program.obj` (Windows uses clang, not `llc` which may be absent)
3. `clang program.obj entry.obj libdesi.lib -o program.exe`
   - `entry.obj` is linked only when the IR contains `@__top__` (no explicit `def main`)
   - MSVC's linker won't pull `main()` from a static lib automatically — `entry.obj` must be explicit

## Platform Portability in the Runtime

All threading/synchronization uses `platform.h` macros (`DESI_MUTEX_*`, `DESI_COND_*`, `DesiPlatformThread`) which expand to either Win32 or pthreads. Do not use `pthread_*` directly in new runtime code.

Windows-specific guards to be aware of:
- `#define _USE_MATH_DEFINES` before `<math.h>` for `M_PI` / `M_E`
- `#define strcasecmp _stricmp` / `strncasecmp _strnicmp`
- `setenv()` → `_putenv_s()` on Windows
- `strptime()` is not in MSVC — a minimal stub lives in `time.c`
- `MSG_DONTWAIT` (POSIX) → `ioctlsocket(FIONBIO)` on Windows
- MSVC rejects variable-length arrays; size them at runtime with `malloc`
- `__thread` is GCC/Clang only — use `DESI_THREAD_LOCAL` from `platform.h`,
  which expands to `__declspec(thread)` under MSVC

## The database runtime

`compiler/runtime/db/*.c` builds on Windows. Two pieces make that work:

- **`db/db_socket.h`** — the slice of the sockets API the PostgreSQL, MySQL
  and Redis drivers need, per platform. WinSock2 on Windows with shims for
  the calls that differ: `closesocket` for `close`, `recv`/`send` for
  `read`/`write` (a `SOCKET` is not a file descriptor), `ioctlsocket(FIONBIO)`
  for `fcntl(O_NONBLOCK)`, `WSAPoll` for `poll`, and `WSAGetLastError` for
  `errno`. The drivers include this instead of `<sys/socket.h>` and friends.
- **`platform.h`** — `pool.c` uses `DesiPlatformMutex`/`DesiPlatformCond`
  rather than pthreads, including `desi_cond_timedwait_ms()` because Win32
  takes a relative timeout where pthreads takes an absolute deadline.

Two Windows-only differences worth knowing when touching this code:
`SO_RCVTIMEO` takes a `DWORD` of milliseconds, not a `struct timeval`; and
`migrate.c` enumerates migration files with `FindFirstFile` since there is no
`dirent.h`.

## Not available on Windows

- **`desic watch`** — hot reload needs Unix signals (`SIGUSR1`) and `.so`
  reloading. `watch_cmd_windows.go` is a stub that prints a message and
  exits 1; the real implementation in `watch_cmd.go` is `//go:build !windows`.
  Note that the no-flag mode is only "type-check on change" and does not
  actually need either of those, so a Windows watcher is feasible if someone
  wants it. User-facing note lives in `book/docs/reference/desic-cli.md`.

Everything else — `run`, `build`, `test`, `check`, `fmt`, `doc`, `desirepl`,
`desifmt`, `desilsp` — works on Windows.

## macOS / Linux

Use `make` as usual. The Makefile excludes `desi_host.c` but all other runtime files compile on Unix.
