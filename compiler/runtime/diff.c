/*
 * diff.c — Text diffing for Desi stdlib
 *
 * Implements line-based diff using a simple LCS (Longest Common Subsequence)
 * algorithm. Produces unified diff output.
 *
 * Public API:
 *   __diff_unified(old_text, new_text)              → unified diff string
 *   __diff_lines(old_text, new_text)                → line-by-line "+/-/ " prefixed
 *   __diff_equal(a, b)                              → 1 if texts are identical
 *   __diff_count_changes(old_text, new_text)         → number of changed lines
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>

/* ---- Split text into lines ---- */

typedef struct {
    char** lines;
    int count;
} LineArray;

static LineArray split_lines(const char* text) {
    LineArray la = {NULL, 0};
    if (!text || !text[0]) return la;

    int cap = 64;
    la.lines = (char**)malloc(sizeof(char*) * cap);

    const char* p = text;
    while (*p) {
        const char* eol = strchr(p, '\n');
        if (!eol) eol = p + strlen(p);
        if (la.count >= cap) {
            cap *= 2;
            la.lines = (char**)realloc(la.lines, sizeof(char*) * cap);
        }
        size_t len = eol - p;
        la.lines[la.count] = (char*)malloc(len + 1);
        memcpy(la.lines[la.count], p, len);
        la.lines[la.count][len] = '\0';
        la.count++;
        p = *eol ? eol + 1 : eol;
    }
    return la;
}

static void free_lines(LineArray* la) {
    for (int i = 0; i < la->count; i++) free(la->lines[i]);
    free(la->lines);
    la->lines = NULL;
    la->count = 0;
}

/* ---- LCS table ---- */

static int** lcs_table(LineArray* a, LineArray* b) {
    int m = a->count, n = b->count;
    int** dp = (int**)malloc(sizeof(int*) * (m + 1));
    for (int i = 0; i <= m; i++) {
        dp[i] = (int*)calloc(n + 1, sizeof(int));
    }
    for (int i = 1; i <= m; i++) {
        for (int j = 1; j <= n; j++) {
            if (strcmp(a->lines[i-1], b->lines[j-1]) == 0) {
                dp[i][j] = dp[i-1][j-1] + 1;
            } else {
                dp[i][j] = dp[i-1][j] > dp[i][j-1] ? dp[i-1][j] : dp[i][j-1];
            }
        }
    }
    return dp;
}

static void free_dp(int** dp, int rows) {
    for (int i = 0; i <= rows; i++) free(dp[i]);
    free(dp);
}

/* ============================================================
 * Public API
 * ============================================================ */

char* __diff_lines(const char* old_text, const char* new_text) {
    LineArray a = split_lines(old_text);
    LineArray b = split_lines(new_text);

    int m = a.count, n = b.count;
    int** dp = lcs_table(&a, &b);

    /* Backtrace to produce diff */
    typedef struct { char type; int idx; } DiffEntry;
    int max_entries = m + n + 1;
    DiffEntry* entries = (DiffEntry*)malloc(sizeof(DiffEntry) * max_entries);
    int entry_count = 0;

    int i = m, j = n;
    while (i > 0 || j > 0) {
        if (i > 0 && j > 0 && strcmp(a.lines[i-1], b.lines[j-1]) == 0) {
            entries[entry_count].type = ' ';
            entries[entry_count].idx = i - 1;
            entry_count++;
            i--; j--;
        } else if (j > 0 && (i == 0 || dp[i][j-1] >= dp[i-1][j])) {
            entries[entry_count].type = '+';
            entries[entry_count].idx = j - 1;
            entry_count++;
            j--;
        } else {
            entries[entry_count].type = '-';
            entries[entry_count].idx = i - 1;
            entry_count++;
            i--;
        }
    }

    /* Build output (entries are in reverse order) */
    size_t cap = 4096, len = 0;
    char* out = (char*)malloc(cap);
    out[0] = '\0';

    for (int k = entry_count - 1; k >= 0; k--) {
        const char* line_text;
        if (entries[k].type == '+') {
            line_text = b.lines[entries[k].idx];
        } else {
            line_text = a.lines[entries[k].idx];
        }
        size_t llen = strlen(line_text);
        while (len + llen + 4 >= cap) { cap *= 2; out = (char*)realloc(out, cap); }
        out[len++] = entries[k].type;
        out[len++] = ' ';
        memcpy(out + len, line_text, llen);
        len += llen;
        out[len++] = '\n';
    }
    out[len] = '\0';

    free(entries);
    free_dp(dp, m);
    free_lines(&a);
    free_lines(&b);
    return out;
}

char* __diff_unified(const char* old_text, const char* new_text) {
    /* Produce unified diff with --- +++ headers */
    char* lines_diff = __diff_lines(old_text, new_text);

    size_t hdr_size = 64 + strlen(lines_diff);
    char* out = (char*)malloc(hdr_size);
    int len = 0;
    len += snprintf(out + len, hdr_size - len, "--- a\n+++ b\n");
    memcpy(out + len, lines_diff, strlen(lines_diff) + 1);
    len += strlen(lines_diff);

    free(lines_diff);
    return out;
}

int32_t __diff_equal(const char* a, const char* b) {
    if (!a && !b) return 1;
    if (!a || !b) return 0;
    return strcmp(a, b) == 0 ? 1 : 0;
}

int32_t __diff_count_changes(const char* old_text, const char* new_text) {
    char* d = __diff_lines(old_text, new_text);
    int count = 0;
    for (const char* p = d; *p; p++) {
        if ((*p == '+' || *p == '-') && (p == d || *(p-1) == '\n')) {
            count++;
        }
    }
    free(d);
    return count;
}
