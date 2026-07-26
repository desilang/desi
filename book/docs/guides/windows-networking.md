# Windows: `dial failed` on localhost

If a Desi program that connects to `127.0.0.1` intermittently fails —
`net.dial` returning `-1`, or a server that never accepts — the cause is
usually **local port exhaustion on Windows**, not your program.

This page explains how to confirm that, because the symptom looks like a
bug in your code and is not one.

## What actually happens

When you `connect()` without binding first, the OS picks a local port for
you from the **dynamic port range** — by default `49152–65535`, so 16,384
ports total:

```
netsh int ipv4 show dynamicport tcp
```

Every closed TCP connection then sits in **`TIME_WAIT`** for ~120 seconds
before its port can be reused. That is required by TCP, not a leak. But it
means a busy machine can hold tens of thousands of `TIME_WAIT` entries at
once, and once the pool is starved, `connect()` fails with
**`WSAEADDRINUSE` (10048)** — "address already in use" — even though the
*server* port is free and listening.

The error name is misleading: it is the **client's** local port that could
not be allocated, not the address you are connecting to.

## Confirming it

Count the sockets parked in `TIME_WAIT`:

```powershell
(netstat -ano -p tcp | Select-String "TIME_WAIT").Count
```

Against a 16,384-port range, a five-figure number means the pool is
starved. To find what is responsible, group by remote port:

```powershell
netstat -ano -p tcp | Select-String "TIME_WAIT" | ForEach-Object {
  if ($_ -match '\d+\.\d+\.\d+\.\d+:(\d+)\s+\d+\.\d+\.\d+\.\d+:(\d+)\s+TIME_WAIT') { $matches[2] }
} | Group-Object | Sort-Object Count -Descending | Select-Object -First 5
```

If one remote port dominates, a background service is churning
connections. Identify its owner:

```powershell
netstat -ano -p tcp | Select-String ":<PORT>\s+LISTENING"
Get-Process -Id <PID>
```

On one development machine this surfaced a vendor agent holding **~7,100
loopback connections** to a single port, keeping the ephemeral pool ~82%
exhausted indefinitely. No amount of application-side retrying fixes that;
the ports genuinely do not exist.

## This is not language-specific

It is an OS-level condition, and every runtime hits it identically. The
same connect/accept sequence, written four ways and run on a machine in
the state described above:

| Implementation | Completed | Could not get a local port |
|---|---|---|
| Desi (`net` module) | 1 / 8 | 7 |
| C, raw WinSock2 | 1 / 8 | 7 |
| Go (`net.Dial`) | 0 / 8 | 8 |
| .NET (`TcpClient`) | 0 / 8 | 8 |

The point is **not** that one language beats another — at this sample size
the differences are noise. The point is that all four behave the same. If
you see this in Desi, you would see it in C, Go, or .NET on the same
machine. Desi's socket layer is a thin wrapper over the platform sockets
and inherits platform behavior exactly.

## What Desi does about it

`net.dial` retries a bounded number of times when Windows reports
`WSAEADDRINUSE`, because a fresh socket gets a different local port. This
absorbs the *transient* collisions a moderately busy machine produces —
including the ones a repeated test run creates against itself. It cannot
manufacture ports on a machine that has none.

The retry is narrow on purpose: it triggers only on `WSAEADDRINUSE`. Any
real failure — connection refused, host unreachable — returns immediately
rather than spinning. On macOS and Linux the single-attempt behavior is
unchanged.

## Fixing your machine

- **Wait.** `TIME_WAIT` entries expire in ~120 seconds. If your own test
  loop starved the pool, it recovers on its own.
- **Stop the offending service**, if you identified one above and do not
  need it running.
- **Widen the dynamic range** (administrator, system-wide):
  ```
  netsh int ipv4 set dynamicport tcp start=10000 num=55535
  ```
- **In your own code**, prefer one long-lived connection over many
  short-lived ones. Connection churn is what consumes the pool.

## Write networking code that fails cleanly

A blocking `accept()` waits forever for a peer that will never arrive, so
an unchecked `dial` failure becomes a hang rather than an error. Always
check the descriptor:

```desi
let srv = net.listen("127.0.0.1", 19876)
if srv < 0:
	print("listen failed")
	return 1

let client = net.dial("127.0.0.1", 19876)
if client < 0:
	# No local socket available — bail out instead of blocking forever
	# in accept() waiting for a peer that will never connect.
	print("dial failed")
	net.close(srv)
	return 1
```

`examples/429_net_module.desi` in the repository follows this pattern.
