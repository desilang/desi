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
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <signal.h>
#include <stdint.h>

/* ---- Callback table ---- */
typedef void (*DesiSignalHandler)(void);

#define MAX_SIGNALS 32
static DesiSignalHandler _signal_handlers[MAX_SIGNALS] = {0};

static void signal_dispatch(int signum) {
    if (signum >= 0 && signum < MAX_SIGNALS && _signal_handlers[signum]) {
        _signal_handlers[signum]();
    }
}

/* ============================================================
 * Public API
 * ============================================================ */

void __signal_on(int signum, DesiSignalHandler handler) {
    if (signum < 0 || signum >= MAX_SIGNALS) return;
    _signal_handlers[signum] = handler;

    struct sigaction sa;
    memset(&sa, 0, sizeof(sa));
    sa.sa_handler = signal_dispatch;
    sigemptyset(&sa.sa_mask);
    sa.sa_flags = SA_RESTART;
    sigaction(signum, &sa, NULL);
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
        case SIGHUP:    return "SIGHUP";
        case SIGQUIT:   return "SIGQUIT";
        case SIGUSR1:   return "SIGUSR1";
        case SIGUSR2:   return "SIGUSR2";
        case SIGALRM:   return "SIGALRM";
        case SIGPIPE:   return "SIGPIPE";
        case SIGCHLD:   return "SIGCHLD";
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
