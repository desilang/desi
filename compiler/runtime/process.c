/*
 * process.c — Subprocess execution for Desi stdlib
 *
 * Run external commands, capture stdout/stderr, set timeouts.
 *
 * Public API:
 *   __process_run(cmd, args[], argc)           → run and wait
 *   __process_run_shell(cmd_str)               → run via /bin/sh -c
 *   __process_get_stdout(result)               → captured stdout
 *   __process_get_stderr(result)               → captured stderr
 *   __process_get_exit_code(result)            → exit code
 *   __process_get_ok(result)                   → exit code == 0
 *   __process_output(cmd, args[], argc)        → shortcut: run + return stdout
 *   __process_free(result)                     → free result
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>
#ifdef _WIN32
  #include <windows.h>
  #include <tlhelp32.h>   /* CreateToolhelp32Snapshot for ppid */
  #include <process.h>    /* _getpid */
#else
  #include <unistd.h>
  #include <sys/wait.h>
  #include <sys/time.h>
  #include <sys/select.h>
  #include <fcntl.h>
  #include <errno.h>
  #include <signal.h>
#endif

typedef struct {
    char*   stdout_buf;
    char*   stderr_buf;
    int32_t exit_code;
} ProcessResult;

#ifdef _WIN32
/* ============================================================
 * Core (Windows): CreateProcess + anonymous pipes + capture
 * ============================================================ */

/* Append one argv element to a command line using Windows quoting rules
 * (quote when needed; backslashes before a quote are doubled). */
static void win_append_arg(char** buf, size_t* len, size_t* cap, const char* arg) {
    size_t need = strlen(arg) * 2 + 4;
    while (*len + need >= *cap) { *cap *= 2; *buf = (char*)realloc(*buf, *cap); }

    if (*len > 0) (*buf)[(*len)++] = ' ';

    int needs_quotes = (*arg == '\0') || strpbrk(arg, " \t\"") != NULL;
    if (!needs_quotes) {
        size_t alen = strlen(arg);
        memcpy(*buf + *len, arg, alen);
        *len += alen;
        return;
    }

    (*buf)[(*len)++] = '"';
    size_t backslashes = 0;
    for (const char* p = arg; *p; p++) {
        if (*p == '\\') {
            backslashes++;
        } else if (*p == '"') {
            for (size_t i = 0; i < backslashes * 2 + 1; i++) (*buf)[(*len)++] = '\\';
            backslashes = 0;
            (*buf)[(*len)++] = '"';
            continue;
        } else {
            for (size_t i = 0; i < backslashes; i++) (*buf)[(*len)++] = '\\';
            backslashes = 0;
        }
        if (*p == '\\') continue; /* emitted when we know what follows */
        (*buf)[(*len)++] = *p;
    }
    for (size_t i = 0; i < backslashes * 2; i++) (*buf)[(*len)++] = '\\';
    (*buf)[(*len)++] = '"';
}

static char* win_build_cmdline(const char** argv) {
    size_t cap = 256, len = 0;
    char* buf = (char*)malloc(cap);
    buf[0] = '\0';
    for (int i = 0; argv[i]; i++) {
        win_append_arg(&buf, &len, &cap, argv[i]);
    }
    buf[len] = '\0';
    return buf;
}

/* Drain whatever is currently available from a pipe (non-blocking). */
static void win_drain_pipe(HANDLE h, char** buf, size_t* len, size_t* cap) {
    DWORD avail = 0;
    while (PeekNamedPipe(h, NULL, 0, NULL, &avail, NULL) && avail > 0) {
        while (*len + avail + 1 >= *cap) { *cap *= 2; *buf = (char*)realloc(*buf, *cap); }
        DWORD got = 0;
        if (!ReadFile(h, *buf + *len, avail, &got, NULL) || got == 0) break;
        *len += got;
        avail = 0;
    }
}

/* Run cmdline (modified in place by CreateProcess), capture stdout/stderr.
 * timeout_ms < 0 means no timeout. */
static ProcessResult* win_exec_cmdline(char* cmdline, int timeout_ms) {
    ProcessResult* r = (ProcessResult*)calloc(1, sizeof(ProcessResult));
    r->exit_code = -1;

    SECURITY_ATTRIBUTES sa;
    sa.nLength = sizeof(sa);
    sa.bInheritHandle = TRUE;
    sa.lpSecurityDescriptor = NULL;

    HANDLE out_r = NULL, out_w = NULL, err_r = NULL, err_w = NULL;
    if (!CreatePipe(&out_r, &out_w, &sa, 0) || !CreatePipe(&err_r, &err_w, &sa, 0)) {
        r->stdout_buf = strdup("");
        r->stderr_buf = strdup("pipe creation failed");
        return r;
    }
    /* Parent ends must not be inherited by the child */
    SetHandleInformation(out_r, HANDLE_FLAG_INHERIT, 0);
    SetHandleInformation(err_r, HANDLE_FLAG_INHERIT, 0);

    STARTUPINFOA si;
    memset(&si, 0, sizeof(si));
    si.cb = sizeof(si);
    si.dwFlags = STARTF_USESTDHANDLES;
    si.hStdOutput = out_w;
    si.hStdError = err_w;
    si.hStdInput = GetStdHandle(STD_INPUT_HANDLE);

    PROCESS_INFORMATION pi;
    memset(&pi, 0, sizeof(pi));

    BOOL ok = CreateProcessA(NULL, cmdline, NULL, NULL, TRUE,
                             CREATE_NO_WINDOW, NULL, NULL, &si, &pi);
    CloseHandle(out_w);
    CloseHandle(err_w);

    if (!ok) {
        CloseHandle(out_r);
        CloseHandle(err_r);
        r->stdout_buf = strdup("");
        r->stderr_buf = strdup("exec failed: CreateProcess error");
        r->exit_code = 127;
        return r;
    }

    size_t out_cap = 4096, out_len = 0;
    char* out_buf = (char*)malloc(out_cap);
    size_t err_cap = 4096, err_len = 0;
    char* err_buf = (char*)malloc(err_cap);

    DWORD start_ticks = GetTickCount();
    int timed_out = 0;

    for (;;) {
        win_drain_pipe(out_r, &out_buf, &out_len, &out_cap);
        win_drain_pipe(err_r, &err_buf, &err_len, &err_cap);

        DWORD wait = WaitForSingleObject(pi.hProcess, 20);
        if (wait == WAIT_OBJECT_0) {
            /* Process exited — drain any remaining output */
            win_drain_pipe(out_r, &out_buf, &out_len, &out_cap);
            win_drain_pipe(err_r, &err_buf, &err_len, &err_cap);
            break;
        }
        if (timeout_ms >= 0 && (int)(GetTickCount() - start_ticks) >= timeout_ms) {
            timed_out = 1;
            TerminateProcess(pi.hProcess, 1);
            WaitForSingleObject(pi.hProcess, 2000);
            break;
        }
    }

    out_buf[out_len] = '\0';
    err_buf[err_len] = '\0';
    CloseHandle(out_r);
    CloseHandle(err_r);

    if (timed_out) {
        r->exit_code = -1;
        char timeout_msg[128];
        snprintf(timeout_msg, sizeof(timeout_msg),
                 "process killed: timeout after %d seconds", timeout_ms / 1000);
        size_t msg_len = strlen(timeout_msg);
        if (err_len + msg_len + 2 >= err_cap) {
            err_cap = err_len + msg_len + 2;
            err_buf = (char*)realloc(err_buf, err_cap);
        }
        if (err_len > 0) err_buf[err_len++] = '\n';
        memcpy(err_buf + err_len, timeout_msg, msg_len + 1);
    } else {
        DWORD code = 0;
        if (GetExitCodeProcess(pi.hProcess, &code)) {
            r->exit_code = (int32_t)code;
        }
    }

    CloseHandle(pi.hProcess);
    CloseHandle(pi.hThread);

    r->stdout_buf = out_buf;
    r->stderr_buf = err_buf;
    return r;
}

static ProcessResult* process_exec(const char** argv) {
    char* cmdline = win_build_cmdline(argv);
    ProcessResult* r = win_exec_cmdline(cmdline, -1);
    free(cmdline);
    return r;
}

static ProcessResult* process_exec_timeout(const char** argv, int timeout_secs) {
    char* cmdline = win_build_cmdline(argv);
    ProcessResult* r = win_exec_cmdline(cmdline, timeout_secs * 1000);
    free(cmdline);
    return r;
}

/* Build "cmd.exe /d /s /c "<cmd>"" for shell execution */
static char* win_shell_cmdline(const char* cmd_str) {
    size_t len = strlen(cmd_str) + 32;
    char* cl = (char*)malloc(len);
    snprintf(cl, len, "cmd.exe /d /s /c \"%s\"", cmd_str);
    return cl;
}

#else
/* ============================================================
 * Core: fork + exec + capture
 * ============================================================ */

static ProcessResult* process_exec(const char** argv) {
    ProcessResult* r = (ProcessResult*)calloc(1, sizeof(ProcessResult));
    r->exit_code = -1;

    int stdout_pipe[2], stderr_pipe[2];
    if (pipe(stdout_pipe) != 0 || pipe(stderr_pipe) != 0) {
        r->stdout_buf = strdup("");
        r->stderr_buf = strdup("pipe creation failed");
        return r;
    }

    pid_t pid = fork();
    if (pid < 0) {
        r->stdout_buf = strdup("");
        r->stderr_buf = strdup("fork failed");
        close(stdout_pipe[0]); close(stdout_pipe[1]);
        close(stderr_pipe[0]); close(stderr_pipe[1]);
        return r;
    }

    if (pid == 0) {
        /* Child */
        close(stdout_pipe[0]);
        close(stderr_pipe[0]);
        dup2(stdout_pipe[1], STDOUT_FILENO);
        dup2(stderr_pipe[1], STDERR_FILENO);
        close(stdout_pipe[1]);
        close(stderr_pipe[1]);
        execvp(argv[0], (char* const*)argv);
        /* If exec fails */
        fprintf(stderr, "exec failed: %s\n", strerror(errno));
        _exit(127);
    }

    /* Parent */
    close(stdout_pipe[1]);
    close(stderr_pipe[1]);

    /* Read stdout */
    size_t out_cap = 4096, out_len = 0;
    char* out_buf = (char*)malloc(out_cap);
    ssize_t n;
    while ((n = read(stdout_pipe[0], out_buf + out_len, out_cap - out_len)) > 0) {
        out_len += n;
        if (out_len >= out_cap) {
            out_cap *= 2;
            out_buf = (char*)realloc(out_buf, out_cap);
        }
    }
    close(stdout_pipe[0]);
    out_buf[out_len] = '\0';

    /* Read stderr */
    size_t err_cap = 4096, err_len = 0;
    char* err_buf = (char*)malloc(err_cap);
    while ((n = read(stderr_pipe[0], err_buf + err_len, err_cap - err_len)) > 0) {
        err_len += n;
        if (err_len >= err_cap) {
            err_cap *= 2;
            err_buf = (char*)realloc(err_buf, err_cap);
        }
    }
    close(stderr_pipe[0]);
    err_buf[err_len] = '\0';

    /* Wait for child */
    int status;
    waitpid(pid, &status, 0);
    if (WIFEXITED(status)) {
        r->exit_code = WEXITSTATUS(status);
    } else if (WIFSIGNALED(status)) {
        r->exit_code = 128 + WTERMSIG(status);
    }

    r->stdout_buf = out_buf;
    r->stderr_buf = err_buf;
    return r;
}
#endif /* !_WIN32 (fork/exec core) */

/* ============================================================
 * Public API
 * ============================================================ */

/* Forward declaration */
void __process_free(ProcessResult* r);

ProcessResult* __process_run(const char* cmd, const char** args, int argc) {
    /* Build argv: [cmd, args..., NULL] */
    const char** argv = (const char**)malloc(sizeof(char*) * (argc + 2));
    argv[0] = cmd;
    for (int i = 0; i < argc; i++) argv[i + 1] = args[i];
    argv[argc + 1] = NULL;

    ProcessResult* r = process_exec(argv);
    free(argv);
    return r;
}

ProcessResult* __process_run_shell(const char* cmd_str) {
#ifdef _WIN32
    char* cl = win_shell_cmdline(cmd_str);
    ProcessResult* r = win_exec_cmdline(cl, -1);
    free(cl);
    return r;
#else
    const char* argv[] = {"/bin/sh", "-c", cmd_str, NULL};
    return process_exec(argv);
#endif
}

const char* __process_get_stdout(ProcessResult* r) {
    return r ? r->stdout_buf : "";
}

const char* __process_get_stderr(ProcessResult* r) {
    return r ? r->stderr_buf : "";
}

int32_t __process_get_exit_code(ProcessResult* r) {
    return r ? r->exit_code : -1;
}

int32_t __process_get_ok(ProcessResult* r) {
    return r ? (r->exit_code == 0 ? 1 : 0) : 0;
}

/* Shortcut: run command and return stdout string (caller frees) */
char* __process_output(const char* cmd, const char** args, int argc) {
    ProcessResult* r = __process_run(cmd, args, argc);
    char* out = strdup(r->stdout_buf);
    __process_free(r);
    return out;
}

/* Shortcut: run shell command and return stdout */
char* __process_shell_output(const char* cmd_str) {
    ProcessResult* r = __process_run_shell(cmd_str);
    char* out = strdup(r->stdout_buf);
    __process_free(r);
    return out;
}

void __process_free(ProcessResult* r) {
    if (!r) return;
    free(r->stdout_buf);
    free(r->stderr_buf);
    free(r);
}

/* ============================================================
 * Process info
 * ============================================================ */

int32_t __process_pid(void) {
#ifdef _WIN32
    return (int32_t)GetCurrentProcessId();
#else
    return (int32_t)getpid();
#endif
}

int32_t __process_ppid(void) {
#ifdef _WIN32
    DWORD self = GetCurrentProcessId();
    DWORD parent = 0;
    HANDLE snap = CreateToolhelp32Snapshot(TH32CS_SNAPPROCESS, 0);
    if (snap != INVALID_HANDLE_VALUE) {
        PROCESSENTRY32 pe;
        pe.dwSize = sizeof(pe);
        if (Process32First(snap, &pe)) {
            do {
                if (pe.th32ProcessID == self) {
                    parent = pe.th32ParentProcessID;
                    break;
                }
            } while (Process32Next(snap, &pe));
        }
        CloseHandle(snap);
    }
    return (int32_t)parent;
#else
    return (int32_t)getppid();
#endif
}

/* ============================================================
 * Timeout execution: kill child after N seconds
 * ============================================================ */

#ifndef _WIN32
static ProcessResult* process_exec_timeout(const char** argv, int timeout_secs) {
    ProcessResult* r = (ProcessResult*)calloc(1, sizeof(ProcessResult));
    r->exit_code = -1;

    int stdout_pipe[2], stderr_pipe[2];
    if (pipe(stdout_pipe) != 0 || pipe(stderr_pipe) != 0) {
        r->stdout_buf = strdup("");
        r->stderr_buf = strdup("pipe creation failed");
        return r;
    }

    pid_t pid = fork();
    if (pid < 0) {
        r->stdout_buf = strdup("");
        r->stderr_buf = strdup("fork failed");
        close(stdout_pipe[0]); close(stdout_pipe[1]);
        close(stderr_pipe[0]); close(stderr_pipe[1]);
        return r;
    }

    if (pid == 0) {
        /* Child */
        close(stdout_pipe[0]);
        close(stderr_pipe[0]);
        dup2(stdout_pipe[1], STDOUT_FILENO);
        dup2(stderr_pipe[1], STDERR_FILENO);
        close(stdout_pipe[1]);
        close(stderr_pipe[1]);
        execvp(argv[0], (char* const*)argv);
        fprintf(stderr, "exec failed: %s\n", strerror(errno));
        _exit(127);
    }

    /* Parent */
    close(stdout_pipe[1]);
    close(stderr_pipe[1]);

    /* Read with timeout using select/poll */
    size_t out_cap = 4096, out_len = 0;
    char* out_buf = (char*)malloc(out_cap);
    size_t err_cap = 4096, err_len = 0;
    char* err_buf = (char*)malloc(err_cap);

    /* Set an alarm-based deadline */
    struct timeval start, now;
    gettimeofday(&start, NULL);
    int timed_out = 0;

    /* Set pipes non-blocking */
    fcntl(stdout_pipe[0], F_SETFL, O_NONBLOCK);
    fcntl(stderr_pipe[0], F_SETFL, O_NONBLOCK);

    int stdout_open = 1, stderr_open = 1;
    while (stdout_open || stderr_open) {
        gettimeofday(&now, NULL);
        int elapsed = (int)(now.tv_sec - start.tv_sec);
        if (elapsed >= timeout_secs) {
            timed_out = 1;
            kill(pid, SIGKILL);
            break;
        }

        fd_set readfds;
        FD_ZERO(&readfds);
        int maxfd = 0;
        if (stdout_open) { FD_SET(stdout_pipe[0], &readfds); if (stdout_pipe[0] > maxfd) maxfd = stdout_pipe[0]; }
        if (stderr_open) { FD_SET(stderr_pipe[0], &readfds); if (stderr_pipe[0] > maxfd) maxfd = stderr_pipe[0]; }

        struct timeval tv;
        tv.tv_sec = timeout_secs - elapsed;
        tv.tv_usec = 0;
        int sel = select(maxfd + 1, &readfds, NULL, NULL, &tv);
        if (sel <= 0) {
            if (sel == 0) { timed_out = 1; kill(pid, SIGKILL); }
            break;
        }

        if (stdout_open && FD_ISSET(stdout_pipe[0], &readfds)) {
            ssize_t n = read(stdout_pipe[0], out_buf + out_len, out_cap - out_len - 1);
            if (n > 0) {
                out_len += n;
                if (out_len >= out_cap - 1) { out_cap *= 2; out_buf = (char*)realloc(out_buf, out_cap); }
            } else { stdout_open = 0; }
        }
        if (stderr_open && FD_ISSET(stderr_pipe[0], &readfds)) {
            ssize_t n = read(stderr_pipe[0], err_buf + err_len, err_cap - err_len - 1);
            if (n > 0) {
                err_len += n;
                if (err_len >= err_cap - 1) { err_cap *= 2; err_buf = (char*)realloc(err_buf, err_cap); }
            } else { stderr_open = 0; }
        }
    }

    close(stdout_pipe[0]);
    close(stderr_pipe[0]);
    out_buf[out_len] = '\0';
    err_buf[err_len] = '\0';

    int status;
    waitpid(pid, &status, 0);

    if (timed_out) {
        r->exit_code = -1;
        /* Append timeout info to stderr */
        char timeout_msg[128];
        snprintf(timeout_msg, sizeof(timeout_msg), "process killed: timeout after %d seconds", timeout_secs);
        size_t msg_len = strlen(timeout_msg);
        if (err_len + msg_len + 2 < err_cap) {
            if (err_len > 0) err_buf[err_len++] = '\n';
            memcpy(err_buf + err_len, timeout_msg, msg_len + 1);
        } else {
            err_cap = err_len + msg_len + 2;
            err_buf = (char*)realloc(err_buf, err_cap);
            if (err_len > 0) err_buf[err_len++] = '\n';
            memcpy(err_buf + err_len, timeout_msg, msg_len + 1);
        }
    } else {
        if (WIFEXITED(status)) {
            r->exit_code = WEXITSTATUS(status);
        } else if (WIFSIGNALED(status)) {
            r->exit_code = 128 + WTERMSIG(status);
        }
    }

    r->stdout_buf = out_buf;
    r->stderr_buf = err_buf;
    return r;
}
#endif /* !_WIN32 (timeout core) */

/* Run command with timeout (seconds). Returns result handle.
 * If timeout is exceeded, process is killed and exit_code = -1.
 * Python: subprocess.run(timeout=N)
 * Go: exec.CommandContext(ctx) */
ProcessResult* __process_run_timeout(const char* cmd, const char** args, int argc, int timeout_secs) {
    const char** argv = (const char**)malloc(sizeof(char*) * (argc + 2));
    argv[0] = cmd;
    for (int i = 0; i < argc; i++) argv[i + 1] = args[i];
    argv[argc + 1] = NULL;

    ProcessResult* r = process_exec_timeout(argv, timeout_secs);
    free(argv);
    return r;
}

/* Run shell command with timeout (seconds) */
ProcessResult* __process_shell_timeout(const char* cmd_str, int timeout_secs) {
#ifdef _WIN32
    char* cl = win_shell_cmdline(cmd_str);
    ProcessResult* r = win_exec_cmdline(cl, timeout_secs * 1000);
    free(cl);
    return r;
#else
    const char* argv[] = {"/bin/sh", "-c", cmd_str, NULL};
    return process_exec_timeout(argv, timeout_secs);
#endif
}

/* Kill a process by PID. Returns 1 on success, 0 on error.
 * Python: os.kill()
 * Go: process.Kill() */
int __process_kill(int pid_val) {
#ifdef _WIN32
    HANDLE h = OpenProcess(PROCESS_TERMINATE, FALSE, (DWORD)pid_val);
    if (!h) return 0;
    int ok = TerminateProcess(h, 1) ? 1 : 0;
    CloseHandle(h);
    return ok;
#else
    return kill((pid_t)pid_val, SIGKILL) == 0 ? 1 : 0;
#endif
}

/* Send signal to a process by PID. Returns 1 on success.
 * Python: os.kill(pid, signal) */
int __process_signal(int pid_val, int sig) {
#ifdef _WIN32
    /* No POSIX signals on Windows: map the terminating signals
     * (SIGKILL=9, SIGTERM=15) to TerminateProcess; others unsupported. */
    if (sig == 9 || sig == 15) return __process_kill(pid_val);
    return 0;
#else
    return kill((pid_t)pid_val, sig) == 0 ? 1 : 0;
#endif
}

