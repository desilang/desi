// csv.c — CSV parsing and writing for Desi stdlib
// RFC 4180 compliant: handles quoted fields, commas, newlines in fields
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdbool.h>

// Forward declare DesiList
typedef struct {
    void** data;
    size_t length;
    size_t capacity;
    int type_tag;
    char* (*to_str_fn)(void*);
} DesiList;
extern DesiList* list_new(int type_tag, char* (*to_str_fn)(void*));
extern void list_append(DesiList* list, void* item, int type_tag);

// ============================================================
// Internal helpers
// ============================================================

// Parse a single CSV line into a list of strings (fields).
// Handles: quoted fields, escaped quotes (""), commas inside quotes.
static DesiList* parse_csv_line(const char* line, char delim) {
    DesiList* fields = list_new(1, NULL); // type_tag 1 = str
    if (!line) return fields;

    const char* p = line;
    while (*p) {
        // Skip leading whitespace (optional)
        char* field = NULL;
        size_t field_len = 0;
        size_t field_cap = 64;
        field = (char*)malloc(field_cap);
        field[0] = '\0';

        if (*p == '"') {
            // Quoted field
            p++; // skip opening "
            while (*p) {
                if (*p == '"') {
                    if (*(p + 1) == '"') {
                        // Escaped quote ""
                        if (field_len + 1 >= field_cap) {
                            field_cap *= 2;
                            field = (char*)realloc(field, field_cap);
                        }
                        field[field_len++] = '"';
                        p += 2;
                    } else {
                        // End of quoted field
                        p++; // skip closing "
                        break;
                    }
                } else {
                    if (field_len + 1 >= field_cap) {
                        field_cap *= 2;
                        field = (char*)realloc(field, field_cap);
                    }
                    field[field_len++] = *p;
                    p++;
                }
            }
            field[field_len] = '\0';
            // Skip delimiter after quoted field
            if (*p == delim) p++;
        } else {
            // Unquoted field
            while (*p && *p != delim && *p != '\n' && *p != '\r') {
                if (field_len + 1 >= field_cap) {
                    field_cap *= 2;
                    field = (char*)realloc(field, field_cap);
                }
                field[field_len++] = *p;
                p++;
            }
            field[field_len] = '\0';
            if (*p == delim) p++;
        }
        list_append(fields, (void*)field, 1);
    }
    return fields;
}

// ============================================================
// Public API
// ============================================================

// Parse a CSV string into a list of rows (list of list of strings).
DesiList* __csv_parse(const char* data) {
    DesiList* rows = list_new(2, NULL); // type_tag 2 = list
    if (!data || *data == '\0') return rows;

    const char* p = data;
    while (*p) {
        // Find end of line (handle \r\n, \n, \r)
        const char* line_start = p;
        // We need to handle quotes — newlines inside quotes are part of the field
        bool in_quotes = false;
        const char* line_end = p;
        while (*line_end) {
            if (*line_end == '"') {
                in_quotes = !in_quotes;
            } else if (!in_quotes && (*line_end == '\n' || *line_end == '\r')) {
                break;
            }
            line_end++;
        }

        // Extract the line
        size_t line_len = (size_t)(line_end - line_start);
        char* line = (char*)malloc(line_len + 1);
        memcpy(line, line_start, line_len);
        line[line_len] = '\0';

        // Skip empty lines
        if (line_len > 0) {
            DesiList* row = parse_csv_line(line, ',');
            list_append(rows, (void*)row, 2);
        }
        free(line);

        // Advance past newline
        p = line_end;
        if (*p == '\r') p++;
        if (*p == '\n') p++;
    }
    return rows;
}

// Parse CSV with custom delimiter
DesiList* __csv_parse_delim(const char* data, const char* delim) {
    char d = (delim && *delim) ? *delim : ',';
    DesiList* rows = list_new(2, NULL);
    if (!data || *data == '\0') return rows;

    const char* p = data;
    while (*p) {
        bool in_quotes = false;
        const char* line_start = p;
        const char* line_end = p;
        while (*line_end) {
            if (*line_end == '"') in_quotes = !in_quotes;
            else if (!in_quotes && (*line_end == '\n' || *line_end == '\r')) break;
            line_end++;
        }
        size_t line_len = (size_t)(line_end - line_start);
        char* line = (char*)malloc(line_len + 1);
        memcpy(line, line_start, line_len);
        line[line_len] = '\0';
        if (line_len > 0) {
            DesiList* row = parse_csv_line(line, d);
            list_append(rows, (void*)row, 2);
        }
        free(line);
        p = line_end;
        if (*p == '\r') p++;
        if (*p == '\n') p++;
    }
    return rows;
}

// Format a list of rows into a CSV string
char* __csv_format(DesiList* rows) {
    if (!rows) return strdup("");

    size_t buf_cap = 256;
    size_t buf_len = 0;
    char* buf = (char*)malloc(buf_cap);
    buf[0] = '\0';

    for (size_t r = 0; r < rows->length; r++) {
        DesiList* row = (DesiList*)rows->data[r];
        if (!row) continue;

        for (size_t c = 0; c < row->length; c++) {
            const char* field = (const char*)row->data[c];
            if (!field) field = "";

            // Check if field needs quoting
            bool needs_quote = false;
            for (const char* f = field; *f; f++) {
                if (*f == ',' || *f == '"' || *f == '\n' || *f == '\r') {
                    needs_quote = true;
                    break;
                }
            }

            if (needs_quote) {
                // Add opening quote
                while (buf_len + 1 >= buf_cap) { buf_cap *= 2; buf = (char*)realloc(buf, buf_cap); }
                buf[buf_len++] = '"';
                // Add field with escaped quotes
                for (const char* f = field; *f; f++) {
                    if (*f == '"') {
                        while (buf_len + 2 >= buf_cap) { buf_cap *= 2; buf = (char*)realloc(buf, buf_cap); }
                        buf[buf_len++] = '"';
                        buf[buf_len++] = '"';
                    } else {
                        while (buf_len + 1 >= buf_cap) { buf_cap *= 2; buf = (char*)realloc(buf, buf_cap); }
                        buf[buf_len++] = *f;
                    }
                }
                // Add closing quote
                while (buf_len + 1 >= buf_cap) { buf_cap *= 2; buf = (char*)realloc(buf, buf_cap); }
                buf[buf_len++] = '"';
            } else {
                // Add field directly
                size_t flen = strlen(field);
                while (buf_len + flen >= buf_cap) { buf_cap *= 2; buf = (char*)realloc(buf, buf_cap); }
                memcpy(buf + buf_len, field, flen);
                buf_len += flen;
            }

            // Add comma between fields
            if (c < row->length - 1) {
                while (buf_len + 1 >= buf_cap) { buf_cap *= 2; buf = (char*)realloc(buf, buf_cap); }
                buf[buf_len++] = ',';
            }
        }
        // Add newline between rows
        while (buf_len + 1 >= buf_cap) { buf_cap *= 2; buf = (char*)realloc(buf, buf_cap); }
        buf[buf_len++] = '\n';
    }
    buf[buf_len] = '\0';
    return buf;
}

// Read CSV file → list of rows
DesiList* __csv_read_file(const char* path) {
    if (!path) return list_new(2, NULL);
    FILE* f = fopen(path, "rb");
    if (!f) return list_new(2, NULL);
    fseek(f, 0, SEEK_END);
    long size = ftell(f);
    fseek(f, 0, SEEK_SET);
    if (size < 0) { fclose(f); return list_new(2, NULL); }
    char* data = (char*)malloc((size_t)size + 1);
    if (!data) { fclose(f); return list_new(2, NULL); }
    size_t nread = fread(data, 1, (size_t)size, f);
    fclose(f);
    data[nread] = '\0';
    DesiList* result = __csv_parse(data);
    free(data);
    return result;
}

// Write rows to CSV file
int __csv_write_file(const char* path, DesiList* rows) {
    if (!path || !rows) return -1;
    char* csv_str = __csv_format(rows);
    FILE* f = fopen(path, "wb");
    if (!f) { free(csv_str); return -1; }
    size_t len = strlen(csv_str);
    size_t written = fwrite(csv_str, 1, len, f);
    fclose(f);
    free(csv_str);
    return (written == len) ? 0 : -1;
}
