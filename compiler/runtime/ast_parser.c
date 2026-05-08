/*
 * ast_parser.c — Runtime AST parser bindings for Desi
 *
 * Provides C functions that expose AST parsing to Desi code at runtime.
 * This is the foundation for `import ast` and v0.2.0 compile-time macros.
 *
 * IMPLEMENTATION NOTE:
 * In v0.1.0, this provides a simplified AST representation using string-based
 * node types. The real AST (from the Go compiler) is not directly exposed;
 * instead, we re-parse .desi files using a minimal C tokenizer to build
 * a lightweight tree. Full AST fidelity comes in v0.2.0 with the Go-to-C
 * bridge for the real parser.
 *
 * Public API:
 *   __ast_parse_file(path)        → opaque AST handle (int64)
 *   __ast_node_type(handle)       → string type name
 *   __ast_node_children_count(h)  → int count
 *   __ast_node_name(handle)       → identifier name (or "")
 *   __ast_node_line(handle)       → line number
 *   __ast_node_col(handle)        → column number
 *   __ast_free(handle)            → free the AST tree
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>

/* ---------- Lightweight AST node ---------- */

#define AST_MAX_CHILDREN 64

typedef struct ASTNode {
    char            type[32];           // "Module", "FuncDecl", "ForStmt", etc.
    char            name[128];          // identifier name (empty if unnamed)
    int             line;
    int             col;
    struct ASTNode *children[AST_MAX_CHILDREN];
    int             child_count;
} ASTNode;

/* ---------- Handle table ---------- */

#define AST_MAX_HANDLES 256
static ASTNode *ast_handles[AST_MAX_HANDLES];
static int ast_handle_count = 0;

static int64_t ast_store(ASTNode *node) {
    if (ast_handle_count >= AST_MAX_HANDLES) return -1;
    int idx = ast_handle_count++;
    ast_handles[idx] = node;
    return (int64_t)idx;
}

static ASTNode *ast_get(int64_t handle) {
    if (handle < 0 || handle >= ast_handle_count) return NULL;
    return ast_handles[(int)handle];
}

/* ---------- Minimal tokenizer ---------- */

static ASTNode *ast_new_node(const char *type, const char *name, int line, int col) {
    ASTNode *n = (ASTNode *)calloc(1, sizeof(ASTNode));
    if (!n) return NULL;
    strncpy(n->type, type, sizeof(n->type) - 1);
    if (name) strncpy(n->name, name, sizeof(n->name) - 1);
    n->line = line;
    n->col = col;
    n->child_count = 0;
    return n;
}

static void ast_add_child(ASTNode *parent, ASTNode *child) {
    if (!parent || !child) return;
    if (parent->child_count < AST_MAX_CHILDREN) {
        parent->children[parent->child_count++] = child;
    }
}

static void ast_free_tree(ASTNode *node) {
    if (!node) return;
    for (int i = 0; i < node->child_count; i++) {
        ast_free_tree(node->children[i]);
    }
    free(node);
}

/* ---------- Simplified parser ---------- */
/* Scans a .desi file line by line, identifies top-level declarations,
 * and builds a skeleton AST. Full parsing requires the Go bridge (v0.2.0). */

static ASTNode *parse_desi_file(const char *src, int src_len) {
    ASTNode *module = ast_new_node("Module", "", 1, 1);
    if (!module) return NULL;

    int line = 1;
    int i = 0;
    while (i < src_len) {
        /* Skip whitespace */
        int col = 1;
        while (i < src_len && src[i] == '\t') { col += 4; i++; }
        while (i < src_len && src[i] == ' ') { col++; i++; }

        /* Skip empty lines and comments */
        if (i >= src_len || src[i] == '\n') {
            if (i < src_len) { line++; i++; }
            continue;
        }
        if (src[i] == '#') {
            while (i < src_len && src[i] != '\n') i++;
            if (i < src_len) { line++; i++; }
            continue;
        }

        /* Detect top-level declarations at col=1 */
        if (col == 1) {
            if (strncmp(src + i, "def ", 4) == 0) {
                /* Extract function name */
                int start = i + 4;
                int end = start;
                while (end < src_len && src[end] != '(' && src[end] != ':' && src[end] != '\n') end++;
                char fname[128] = {0};
                int flen = end - start;
                if (flen > 0 && flen < (int)sizeof(fname)) {
                    memcpy(fname, src + start, flen);
                    /* Trim trailing spaces */
                    while (flen > 0 && fname[flen-1] == ' ') fname[--flen] = '\0';
                }
                ASTNode *func = ast_new_node("FuncDecl", fname, line, col);
                ast_add_child(module, func);
            } else if (strncmp(src + i, "class ", 6) == 0) {
                int start = i + 6;
                int end = start;
                while (end < src_len && src[end] != '(' && src[end] != ':' && src[end] != '\n') end++;
                char cname[128] = {0};
                int clen = end - start;
                if (clen > 0 && clen < (int)sizeof(cname)) {
                    memcpy(cname, src + start, clen);
                    while (clen > 0 && cname[clen-1] == ' ') cname[--clen] = '\0';
                }
                ASTNode *cls = ast_new_node("ClassDecl", cname, line, col);
                ast_add_child(module, cls);
            } else if (strncmp(src + i, "struct ", 7) == 0) {
                int start = i + 7;
                int end = start;
                while (end < src_len && src[end] != '(' && src[end] != ':' && src[end] != '\n') end++;
                char sname[128] = {0};
                int slen = end - start;
                if (slen > 0 && slen < (int)sizeof(sname)) {
                    memcpy(sname, src + start, slen);
                    while (slen > 0 && sname[slen-1] == ' ') sname[--slen] = '\0';
                }
                ASTNode *st = ast_new_node("StructDecl", sname, line, col);
                ast_add_child(module, st);
            } else if (strncmp(src + i, "enum ", 5) == 0) {
                int start = i + 5;
                int end = start;
                while (end < src_len && src[end] != '(' && src[end] != ':' && src[end] != '\n') end++;
                char ename[128] = {0};
                int elen = end - start;
                if (elen > 0 && elen < (int)sizeof(ename)) {
                    memcpy(ename, src + start, elen);
                    while (elen > 0 && ename[elen-1] == ' ') ename[--elen] = '\0';
                }
                ASTNode *en = ast_new_node("EnumDecl", ename, line, col);
                ast_add_child(module, en);
            } else if (strncmp(src + i, "import ", 7) == 0) {
                int start = i + 7;
                int end = start;
                while (end < src_len && src[end] != '\n') end++;
                char iname[128] = {0};
                int ilen = end - start;
                if (ilen > 0 && ilen < (int)sizeof(iname)) {
                    memcpy(iname, src + start, ilen);
                    while (ilen > 0 && iname[ilen-1] == ' ') iname[--ilen] = '\0';
                }
                ASTNode *imp = ast_new_node("ImportDecl", iname, line, col);
                ast_add_child(module, imp);
            } else if (strncmp(src + i, "let ", 4) == 0) {
                ASTNode *let = ast_new_node("LetStmt", "", line, col);
                ast_add_child(module, let);
            }
        }

        /* Skip to next line */
        while (i < src_len && src[i] != '\n') i++;
        if (i < src_len) { line++; i++; }
    }

    return module;
}

/* ---------- Public API ---------- */

int64_t __ast_parse_file(const char *path) {
    FILE *f = fopen(path, "rb");
    if (!f) {
        fprintf(stderr, "[ast] error: cannot open '%s'\n", path);
        return -1;
    }
    fseek(f, 0, SEEK_END);
    long len = ftell(f);
    fseek(f, 0, SEEK_SET);

    char *buf = (char *)malloc(len + 1);
    if (!buf) { fclose(f); return -1; }
    fread(buf, 1, len, f);
    buf[len] = '\0';
    fclose(f);

    ASTNode *root = parse_desi_file(buf, (int)len);
    free(buf);
    if (!root) return -1;

    return ast_store(root);
}

const char *__ast_node_type(int64_t handle) {
    ASTNode *n = ast_get(handle);
    return n ? n->type : "";
}

int __ast_node_children_count(int64_t handle) {
    ASTNode *n = ast_get(handle);
    return n ? n->child_count : 0;
}

const char *__ast_node_name(int64_t handle) {
    ASTNode *n = ast_get(handle);
    return n ? n->name : "";
}

int __ast_node_line(int64_t handle) {
    ASTNode *n = ast_get(handle);
    return n ? n->line : 0;
}

int __ast_node_col(int64_t handle) {
    ASTNode *n = ast_get(handle);
    return n ? n->col : 0;
}

void __ast_free(int64_t handle) {
    ASTNode *n = ast_get(handle);
    if (n) {
        ast_free_tree(n);
        ast_handles[(int)handle] = NULL;
    }
}
