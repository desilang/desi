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
#include <unistd.h>
#include <sys/wait.h>
#include <sys/time.h>
#include <sys/select.h>
#include <fcntl.h>
#include <errno.h>
#include <signal.h>

typedef struct {
    char*   stdout_buf;
    char*   stderr_buf;
    int32_t exit_code;
} ProcessResult;

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
    const char* argv[] = {"/bin/sh", "-c", cmd_str, NULL};
    return process_exec(argv);
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
    return (int32_t)getpid();
}

int32_t __process_ppid(void) {
    return (int32_t)getppid();
}

/* ============================================================
 * Timeout execution: kill child after N seconds
 * ============================================================ */

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
    const char* argv[] = {"/bin/sh", "-c", cmd_str, NULL};
    return process_exec_timeout(argv, timeout_secs);
}

/* Kill a process by PID. Returns 1 on success, 0 on error.
 * Python: os.kill()
 * Go: process.Kill() */
int __process_kill(int pid_val) {
    return kill((pid_t)pid_val, SIGKILL) == 0 ? 1 : 0;
}

/* Send signal to a process by PID. Returns 1 on success.
 * Python: os.kill(pid, signal) */
int __process_signal(int pid_val, int sig) {
    return kill((pid_t)pid_val, sig) == 0 ? 1 : 0;
}

