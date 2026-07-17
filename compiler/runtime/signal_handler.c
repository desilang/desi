/*
 * signal_handler.c — OS signal handling for Desi stdlib
 *
 * Register callbacks for Unix signals (SIGINT, SIGTERM, etc.).
 * Uses a simple dispatch table with function pointers.
 *
 * Public API:
 *   __signal_on(signum, handler)     → register handler for signal
 *   __signal_reset(signum)           → reset to default handler
 *   __signal_ignore(signum)          → ignore signal
 *   __signal_raise(signum)           → raise signal to self
 *   __signal_name(signum)            → human-readable signal name
 *
 * Constants:
 *   __signal_SIGINT, __signal_SIGTERM, __signal_SIGHUP, etc.
 *
 * Windows: uses SetConsoleCtrlHandler for CTRL_C / CTRL_BREAK / CTRL_CLOSE
 *          and signal() for the subset MSVC supports (SIGINT, SIGTERM, SIGABRT).
 *          Unix-only signals (SIGHUP, SIGUSR1, etc.) are mapped to no-op
 *          constants so Desi source that references them still compiles.
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <signal.h>
#include <stdint.h>

#ifdef _WIN32
#  define WIN32_LEAN_AND_MEAN
#  include <windows.h>
#endif

/* ---- Callback table ---- */
typedef void (*DesiSignalHandler)(void);

#define MAX_SIGNALS 32
static DesiSignalHandler _signal_handlers[MAX_SIGNALS] = {0};

/* ---- Unix-only signal constants ---- */
#ifdef _WIN32
/* MSVC's <signal.h> only defines SIGINT, SIGTERM, SIGABRT, SIGFPE, SIGILL, SIGSEGV.
   Define the rest as harmless high values so Desi code that references them compiles. */
#  ifndef SIGHUP
#    define SIGHUP   25
#  endif
#  ifndef SIGQUIT
#    define SIGQUIT  26
#  endif
#  ifndef SIGUSR1
#    define SIGUSR1  27
#  endif
#  ifndef SIGUSR2
#    define SIGUSR2  28
#  endif
#  ifndef SIGALRM
#    define SIGALRM  29
#  endif
#  ifndef SIGPIPE
#    define SIGPIPE  30
#  endif
#  ifndef SIGCHLD
#    define SIGCHLD  31
#  endif
#endif

static void signal_dispatch(int signum) {
    if (signum >= 0 && signum < MAX_SIGNALS && _signal_handlers[signum]) {
        _signal_handlers[signum]();
    }
}

#ifdef _WIN32
static BOOL WINAPI console_ctrl_handler(DWORD dwCtrlType) {
    int signum = -1;
    switch (dwCtrlType) {
        case CTRL_C_EVENT:        signum = SIGINT;  break;
        case CTRL_BREAK_EVENT:    signum = SIGINT;  break;
        case CTRL_CLOSE_EVENT:    signum = SIGTERM; break;
        case CTRL_LOGOFF_EVENT:   signum = SIGHUP;  break;
        case CTRL_SHUTDOWN_EVENT: signum = SIGTERM; break;
    }
    if (signum >= 0 && signum < MAX_SIGNALS && _signal_handlers[signum]) {
        _signal_handlers[signum]();
        return TRUE;
    }
    return FALSE;
}

static int _console_handler_installed = 0;

static void ensure_console_handler(void) {
    if (!_console_handler_installed) {
        SetConsoleCtrlHandler(console_ctrl_handler, TRUE);
        _console_handler_installed = 1;
    }
}
#endif /* _WIN32 */

/* ============================================================
 * Public API
 * ============================================================ */

void __signal_on(int signum, DesiSignalHandler handler) {
    if (signum < 0 || signum >= MAX_SIGNALS) return;
    _signal_handlers[signum] = handler;

#ifdef _WIN32
    ensure_console_handler();
    /* For signals MSVC supports natively, also register via signal(). */
    if (signum == SIGINT || signum == SIGTERM || signum == SIGABRT) {
        signal(signum, signal_dispatch);
    }
#else
    struct sigaction sa;
    memset(&sa, 0, sizeof(sa));
    sa.sa_handler = signal_dispatch;
    sigemptyset(&sa.sa_mask);
    sa.sa_flags = SA_RESTART;
    sigaction(signum, &sa, NULL);
#endif
}

void __signal_reset(int signum) {
    if (signum < 0 || signum >= MAX_SIGNALS) return;
    _signal_handlers[signum] = NULL;
    signal(signum, SIG_DFL);
}

void __signal_ignore(int signum) {
    if (signum < 0 || signum >= MAX_SIGNALS) return;
    _signal_handlers[signum] = NULL;
    signal(signum, SIG_IGN);
}

void __signal_raise(int signum) {
    raise(signum);
}

const char* __signal_name(int signum) {
    switch (signum) {
        case SIGINT:    return "SIGINT";
        case SIGTERM:   return "SIGTERM";
#ifndef _WIN32
        case SIGHUP:    return "SIGHUP";
        case SIGQUIT:   return "SIGQUIT";
        case SIGUSR1:   return "SIGUSR1";
        case SIGUSR2:   return "SIGUSR2";
        case SIGALRM:   return "SIGALRM";
        case SIGPIPE:   return "SIGPIPE";
        case SIGCHLD:   return "SIGCHLD";
#else
        case SIGABRT:   return "SIGABRT";
        case SIGHUP:    return "SIGHUP";
        case SIGQUIT:   return "SIGQUIT";
        case SIGUSR1:   return "SIGUSR1";
        case SIGUSR2:   return "SIGUSR2";
        case SIGALRM:   return "SIGALRM";
        case SIGPIPE:   return "SIGPIPE";
        case SIGCHLD:   return "SIGCHLD";
#endif
        default:        return "UNKNOWN";
    }
}

/* Signal number constants (exposed to Desi) */
int32_t __signal_SIGINT(void)  { return SIGINT; }
int32_t __signal_SIGTERM(void) { return SIGTERM; }
int32_t __signal_SIGHUP(void)  { return SIGHUP; }
int32_t __signal_SIGQUIT(void) { return SIGQUIT; }
int32_t __signal_SIGUSR1(void) { return SIGUSR1; }
int32_t __signal_SIGUSR2(void) { return SIGUSR2; }
int32_t __signal_SIGALRM(void) { return SIGALRM; }
int32_t __signal_SIGPIPE(void) { return SIGPIPE; }
