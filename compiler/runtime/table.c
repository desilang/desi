/*
 * table.c — Pretty table printing for Desi stdlib
 *
 * Renders formatted ASCII tables for CLI output.
 *
 * Public API:
 *   __table_new(col_names[], ncols)     → create table with column headers
 *   __table_add_row(t, values[], ncols) → add a data row
 *   __table_render(t)                   → render to string (box-drawing chars)
 *   __table_render_simple(t)            → render with simple ASCII borders
 *   __table_set_align(t, col, align)    → set alignment: 'l', 'r', 'c'
 *   __table_len(t)                      → number of rows
 *   __table_free(t)                     → destroy
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>
#include "list.h"

typedef struct {
    char*** rows;     /* rows[i][j] = cell string */
    int     nrows;
    int     row_cap;
    char**  headers;
    int     ncols;
    int*    col_widths;
    char*   aligns;   /* 'l', 'r', 'c' per column */
} DesiTable;

/* ============================================================
 * Creation
 * ============================================================ */

DesiTable* __table_new(DesiList* col_names, int ncols) {
    DesiTable* t = (DesiTable*)calloc(1, sizeof(DesiTable));
    t->ncols = ncols;
    t->row_cap = 16;
    t->rows = (char***)calloc(t->row_cap, sizeof(char**));
    t->headers = (char**)malloc(sizeof(char*) * ncols);
    t->col_widths = (int*)calloc(ncols, sizeof(int));
    t->aligns = (char*)malloc(ncols);

    for (int i = 0; i < ncols; i++) {
        const char* name = (col_names && i < (int)col_names->length) ?
            (const char*)col_names->data[i] : "";
        t->headers[i] = strdup(name ? name : "");
        t->col_widths[i] = (int)strlen(t->headers[i]);
        t->aligns[i] = 'l'; /* default left align */
    }
    return t;
}

/* ============================================================
 * Add rows
 * ============================================================ */

void __table_add_row(DesiTable* t, DesiList* values, int ncols) {
    if (!t) return;
    if (t->nrows >= t->row_cap) {
        t->row_cap *= 2;
        t->rows = (char***)realloc(t->rows, sizeof(char**) * t->row_cap);
    }
    char** row = (char**)malloc(sizeof(char*) * t->ncols);
    for (int i = 0; i < t->ncols; i++) {
        const char* val = "";
        if (values && i < ncols && i < (int)values->length) {
            val = (const char*)values->data[i];
            if (!val) val = "";
        }
        row[i] = strdup(val);
        int vlen = (int)strlen(val);
        if (vlen > t->col_widths[i]) t->col_widths[i] = vlen;
    }
    t->rows[t->nrows++] = row;
}

void __table_set_align(DesiTable* t, int col, char align) {
    if (t && col >= 0 && col < t->ncols) {
        t->aligns[col] = align;
    }
}

int32_t __table_len(DesiTable* t) { return t ? t->nrows : 0; }

/* ============================================================
 * Render — box-drawing characters (Unicode)
 * ============================================================ */

static void write_padded(char* buf, int* pos, const char* text, int width, char align) {
    int tlen = (int)strlen(text);
    int pad = width - tlen;
    if (pad < 0) pad = 0;

    if (align == 'r') {
        for (int i = 0; i < pad; i++) buf[(*pos)++] = ' ';
        memcpy(buf + *pos, text, tlen); *pos += tlen;
    } else if (align == 'c') {
        int left = pad / 2, right = pad - left;
        for (int i = 0; i < left; i++) buf[(*pos)++] = ' ';
        memcpy(buf + *pos, text, tlen); *pos += tlen;
        for (int i = 0; i < right; i++) buf[(*pos)++] = ' ';
    } else { /* 'l' */
        memcpy(buf + *pos, text, tlen); *pos += tlen;
        for (int i = 0; i < pad; i++) buf[(*pos)++] = ' ';
    }
}

static void write_border(char* buf, int* pos, DesiTable* t,
                          const char* left, const char* mid, const char* right,
                          const char* fill) {
    int flen = (int)strlen(fill);
    int llen = (int)strlen(left);
    int mlen = (int)strlen(mid);
    int rlen = (int)strlen(right);

    memcpy(buf + *pos, left, llen); *pos += llen;
    for (int i = 0; i < t->ncols; i++) {
        if (i > 0) { memcpy(buf + *pos, mid, mlen); *pos += mlen; }
        for (int j = 0; j < t->col_widths[i] + 2; j++) {
            memcpy(buf + *pos, fill, flen); *pos += flen;
        }
    }
    memcpy(buf + *pos, right, rlen); *pos += rlen;
    buf[(*pos)++] = '\n';
}

static void write_data_row(char* buf, int* pos, DesiTable* t, char** cells) {
    buf[(*pos)++] = '|'; buf[(*pos)++] = ' ';
    for (int i = 0; i < t->ncols; i++) {
        if (i > 0) { buf[(*pos)++] = ' '; buf[(*pos)++] = '|'; buf[(*pos)++] = ' '; }
        write_padded(buf, pos, cells[i], t->col_widths[i], t->aligns[i]);
    }
    buf[(*pos)++] = ' '; buf[(*pos)++] = '|'; buf[(*pos)++] = '\n';
}

char* __table_render(DesiTable* t) {
    if (!t) return strdup("");

    /* Estimate buffer size */
    int total_width = 1;
    for (int i = 0; i < t->ncols; i++) total_width += t->col_widths[i] + 3;
    total_width += 1;
    int est = (t->nrows + 4) * (total_width + 10) * 4; /* x4 for UTF-8 */
    char* buf = (char*)malloc(est);
    int pos = 0;

    /* Top border: ┌───┬───┐ */
    write_border(buf, &pos, t, "\xe2\x94\x8c", "\xe2\x94\xac", "\xe2\x94\x90", "\xe2\x94\x80");

    /* Header row */
    write_data_row(buf, &pos, t, t->headers);

    /* Header separator: ├───┼───┤ */
    write_border(buf, &pos, t, "\xe2\x94\x9c", "\xe2\x94\xbc", "\xe2\x94\xa4", "\xe2\x94\x80");

    /* Data rows */
    for (int r = 0; r < t->nrows; r++) {
        write_data_row(buf, &pos, t, t->rows[r]);
    }

    /* Bottom border: └───┴───┘ */
    write_border(buf, &pos, t, "\xe2\x94\x94", "\xe2\x94\xb4", "\xe2\x94\x98", "\xe2\x94\x80");

    buf[pos] = '\0';
    return buf;
}

char* __table_render_simple(DesiTable* t) {
    if (!t) return strdup("");

    int total_width = 1;
    for (int i = 0; i < t->ncols; i++) total_width += t->col_widths[i] + 3;
    total_width += 1;
    int est = (t->nrows + 4) * (total_width + 4);
    char* buf = (char*)malloc(est);
    int pos = 0;

    /* Top border: +---+---+ */
    write_border(buf, &pos, t, "+", "+", "+", "-");
    write_data_row(buf, &pos, t, t->headers);
    write_border(buf, &pos, t, "+", "+", "+", "-");
    for (int r = 0; r < t->nrows; r++) {
        write_data_row(buf, &pos, t, t->rows[r]);
    }
    write_border(buf, &pos, t, "+", "+", "+", "-");

    buf[pos] = '\0';
    return buf;
}

/* ============================================================
 * Cleanup
 * ============================================================ */

void __table_free(DesiTable* t) {
    if (!t) return;
    for (int i = 0; i < t->ncols; i++) free(t->headers[i]);
    free(t->headers);
    for (int r = 0; r < t->nrows; r++) {
        for (int c = 0; c < t->ncols; c++) free(t->rows[r][c]);
        free(t->rows[r]);
    }
    free(t->rows);
    free(t->col_widths);
    free(t->aligns);
    free(t);
}
