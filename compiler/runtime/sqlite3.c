/*
 * sqlite3.c — Embedded SQLite3 wrapper for Desi
 *
 * Uses the SQLite3 amalgamation (sqlite3_amalg.c/h) bundled in the runtime.
 * Zero external dependencies — works on macOS, Linux, and Windows.
 * Cross-platform, cross-architecture (arm64, x86_64, etc.).
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>

// Embedded SQLite3 amalgamation (zero external deps)
#include "sqlite3_amalg.h"

static sqlite3* _db = NULL;
static char _last_error[1024] = {0};

// Simple row storage for query results
#define MAX_COLS 64
#define MAX_ROWS 1024

static char* _result_data[MAX_ROWS][MAX_COLS];
static char* _result_cols[MAX_COLS];
static int _result_nrows = 0;
static int _result_ncols = 0;
static int _result_cursor = 0;

static void clear_results(void) {
    for (int r = 0; r < _result_nrows; r++) {
        for (int c = 0; c < _result_ncols; c++) {
            free(_result_data[r][c]);
            _result_data[r][c] = NULL;
        }
    }
    for (int c = 0; c < _result_ncols; c++) {
        free(_result_cols[c]);
        _result_cols[c] = NULL;
    }
    _result_nrows = 0;
    _result_ncols = 0;
    _result_cursor = 0;
}

// Open a database file
int32_t __sqlite3_open(const char* path) {
    if (_db) sqlite3_close(_db);
    clear_results();
    
    int rc = sqlite3_open(path, &_db);
    if (rc != SQLITE_OK) {
        snprintf(_last_error, sizeof(_last_error), "%s", sqlite3_errmsg(_db));
        sqlite3_close(_db);
        _db = NULL;
        return -1;
    }
    _last_error[0] = '\0';
    return 0;
}

// Close the database
int32_t __sqlite3_close(void) {
    clear_results();
    if (_db) {
        sqlite3_close(_db);
        _db = NULL;
    }
    return 0;
}

// Execute a non-query SQL statement (CREATE, INSERT, UPDATE, DELETE)
int32_t __sqlite3_execute(const char* sql) {
    if (!_db) {
        snprintf(_last_error, sizeof(_last_error), "no database open");
        return -1;
    }
    
    char* errmsg = NULL;
    int rc = sqlite3_exec(_db, sql, NULL, NULL, &errmsg);
    if (rc != SQLITE_OK) {
        snprintf(_last_error, sizeof(_last_error), "%s", errmsg ? errmsg : "unknown error");
        if (errmsg) sqlite3_free(errmsg);
        return -1;
    }
    _last_error[0] = '\0';
    return 0;
}

// Execute a query and store results
int32_t __sqlite3_query(const char* sql) {
    if (!_db) {
        snprintf(_last_error, sizeof(_last_error), "no database open");
        return -1;
    }
    
    clear_results();
    
    sqlite3_stmt* stmt;
    int rc = sqlite3_prepare_v2(_db, sql, -1, &stmt, NULL);
    if (rc != SQLITE_OK) {
        snprintf(_last_error, sizeof(_last_error), "%s", sqlite3_errmsg(_db));
        return -1;
    }
    
    _result_ncols = sqlite3_column_count(stmt);
    for (int i = 0; i < _result_ncols && i < MAX_COLS; i++) {
        const char* name = sqlite3_column_name(stmt, i);
        _result_cols[i] = strdup(name ? name : "");
    }
    
    while ((rc = sqlite3_step(stmt)) == SQLITE_ROW && _result_nrows < MAX_ROWS) {
        for (int i = 0; i < _result_ncols && i < MAX_COLS; i++) {
            const char* val = (const char*)sqlite3_column_text(stmt, i);
            _result_data[_result_nrows][i] = strdup(val ? val : "");
        }
        _result_nrows++;
    }
    
    sqlite3_finalize(stmt);
    _result_cursor = 0;
    _last_error[0] = '\0';
    return _result_nrows;
}

// Get number of rows in last query result
int32_t __sqlite3_row_count(void) {
    return _result_nrows;
}

// Get number of columns in last query result
int32_t __sqlite3_col_count(void) {
    return _result_ncols;
}

// Get column name by index
const char* __sqlite3_col_name(int32_t idx) {
    if (idx < 0 || idx >= _result_ncols) return "";
    return _result_cols[idx] ? _result_cols[idx] : "";
}

// Fetch next row (advance cursor). Returns 1 if row available, 0 if done.
int32_t __sqlite3_fetch_next(void) {
    if (_result_cursor < _result_nrows) {
        _result_cursor++;
        return 1;
    }
    return 0;
}

// Get field value from current row by column index
const char* __sqlite3_get_field(int32_t col) {
    int row = _result_cursor - 1;
    if (row < 0 || row >= _result_nrows || col < 0 || col >= _result_ncols) return "";
    return _result_data[row][col] ? _result_data[row][col] : "";
}

// Get field value from current row by column name
const char* __sqlite3_get_field_by_name(const char* name) {
    for (int c = 0; c < _result_ncols; c++) {
        if (_result_cols[c] && strcmp(_result_cols[c], name) == 0) {
            return __sqlite3_get_field(c);
        }
    }
    return "";
}

// Get last insert rowid
int32_t __sqlite3_last_insert_id(void) {
    if (!_db) return -1;
    return (int32_t)sqlite3_last_insert_rowid(_db);
}

// Get number of rows changed by last INSERT/UPDATE/DELETE
int32_t __sqlite3_changes(void) {
    if (!_db) return 0;
    return (int32_t)sqlite3_changes(_db);
}

// Get last error message
const char* __sqlite3_last_error(void) {
    return _last_error;
}

// Begin transaction
int32_t __sqlite3_begin(void) {
    return __sqlite3_execute("BEGIN");
}

// Commit transaction
int32_t __sqlite3_commit(void) {
    return __sqlite3_execute("COMMIT");
}

// Rollback transaction
int32_t __sqlite3_rollback(void) {
    return __sqlite3_execute("ROLLBACK");
}

