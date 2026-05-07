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
