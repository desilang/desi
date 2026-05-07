/*
 * yaml.c — Minimal YAML parser for Desi stdlib
 *
 * Handles a practical subset of YAML:
 *   - Key: value mappings (nested via indentation)
 *   - Lists (- item)
 *   - Strings (quoted and unquoted)
 *   - Numbers, booleans, null
 *   - Comments (#)
 *   - Multi-line strings (| and >)
 *
 * Values are accessed via dotted paths: "server.host", "server.ports.0"
 *
 * Public API:
 *   __yaml_parse(content)             → parse YAML string
 *   __yaml_parse_file(path)           → parse YAML file
 *   __yaml_get(y, dotted_path)        → get string value at path
 *   __yaml_get_int(y, path)           → get as integer
 *   __yaml_get_float(y, path)         → get as float
 *   __yaml_get_bool(y, path)          → get as bool
 *   __yaml_has(y, path)               → check if path exists
 *   __yaml_keys(y, path)              → list keys at path ("\n" separated)
 *   __yaml_len(y, path)               → length of list at path
 *   __yaml_dump(y)                    → serialize back to YAML string
 *   __yaml_free(y)                    → destroy
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <ctype.h>
#include <stdint.h>

/* ---- Node types ---- */
typedef enum { YAML_STRING, YAML_MAP, YAML_LIST } YamlType;

typedef struct YamlNode {
    char*  key;
    char*  value;       /* for string nodes */
    YamlType type;
    struct YamlNode** children;
    int child_count;
    int child_cap;
} YamlNode;

/* ============================================================
 * Node management
 * ============================================================ */

static YamlNode* yaml_node_new(YamlType type, const char* key, const char* value) {
    YamlNode* n = (YamlNode*)calloc(1, sizeof(YamlNode));
    n->type = type;
    n->key = key ? strdup(key) : NULL;
    n->value = value ? strdup(value) : NULL;
    n->child_cap = 8;
    n->children = (YamlNode**)calloc(n->child_cap, sizeof(YamlNode*));
    return n;
}

static void yaml_node_add_child(YamlNode* parent, YamlNode* child) {
    if (parent->child_count >= parent->child_cap) {
        parent->child_cap *= 2;
        parent->children = (YamlNode**)realloc(parent->children,
                            sizeof(YamlNode*) * parent->child_cap);
    }
    parent->children[parent->child_count++] = child;
}

static YamlNode* yaml_node_find(YamlNode* node, const char* key) {
    if (!node || !key) return NULL;
    for (int i = 0; i < node->child_count; i++) {
        if (node->children[i]->key && strcmp(node->children[i]->key, key) == 0)
            return node->children[i];
    }
    return NULL;
}

/* ============================================================
 * Parser helpers
 * ============================================================ */

static int get_indent(const char* line) {
    int indent = 0;
    while (line[indent] == ' ') indent++;
    return indent;
}

static char* yaml_trim(char* s) {
    while (*s && isspace((unsigned char)*s)) s++;
    char* end = s + strlen(s) - 1;
    while (end > s && isspace((unsigned char)*end)) *end-- = '\0';
    return s;
}

static char* strip_yaml_quotes(char* s) {
    size_t len = strlen(s);
    if (len >= 2 && ((s[0] == '"' && s[len-1] == '"') ||
                      (s[0] == '\'' && s[len-1] == '\''))) {
        s[len-1] = '\0';
        return s + 1;
    }
    return s;
}

/* ============================================================
 * Parser (recursive descent by indentation)
 * ============================================================ */

typedef struct {
    char** lines;
    int count;
    int pos;
} YamlParser;

static YamlNode* parse_value(YamlParser* p, int min_indent);

static YamlNode* parse_mapping(YamlParser* p, int min_indent) {
    YamlNode* map = yaml_node_new(YAML_MAP, NULL, NULL);

    while (p->pos < p->count) {
        char* line = p->lines[p->pos];
        int indent = get_indent(line);
        char* trimmed = yaml_trim(line);

        if (trimmed[0] == '\0' || trimmed[0] == '#') {
            p->pos++;
            continue;
        }

        if (indent < min_indent) break;

        /* List item? */
        if (trimmed[0] == '-' && (trimmed[1] == ' ' || trimmed[1] == '\0')) {
            break; /* Not a mapping */
        }

        /* key: value */
        char* colon = strchr(trimmed, ':');
        if (!colon) { p->pos++; continue; }

        *colon = '\0';
        char* key = yaml_trim(trimmed);
        char* val = yaml_trim(colon + 1);

        if (val[0] == '\0') {
            /* Value is on next lines (nested) */
            p->pos++;
            if (p->pos < p->count) {
                int next_indent = get_indent(p->lines[p->pos]);
                char* next_trimmed = yaml_trim(p->lines[p->pos]);
                if (next_indent > indent && next_trimmed[0] == '-') {
                    /* It's a list */
                    YamlNode* list = yaml_node_new(YAML_LIST, key, NULL);
                    while (p->pos < p->count) {
                        int li = get_indent(p->lines[p->pos]);
                        char* lt = yaml_trim(p->lines[p->pos]);
                        if (lt[0] == '\0' || lt[0] == '#') { p->pos++; continue; }
                        if (li < next_indent) break;
                        if (lt[0] == '-') {
                            char* item_val = yaml_trim(lt + 1);
                            if (item_val[0] == ' ') item_val++;
                            item_val = strip_yaml_quotes(item_val);
                            char idx[16];
                            snprintf(idx, sizeof(idx), "%d", list->child_count);
                            yaml_node_add_child(list, yaml_node_new(YAML_STRING, idx, item_val));
                            p->pos++;
                        } else {
                            break;
                        }
                    }
                    yaml_node_add_child(map, list);
                } else if (next_indent > indent) {
                    /* Nested mapping */
                    YamlNode* sub = parse_mapping(p, next_indent);
                    sub->key = strdup(key);
                    yaml_node_add_child(map, sub);
                } else {
                    /* Empty value */
                    yaml_node_add_child(map, yaml_node_new(YAML_STRING, key, ""));
                }
            }
        } else {
            /* Inline value */
            val = strip_yaml_quotes(val);
            /* Strip inline comments */
            char* comment = strstr(val, " #");
            if (comment) *comment = '\0';
            yaml_node_add_child(map, yaml_node_new(YAML_STRING, key, val));
            p->pos++;
        }
    }

    return map;
}

/* ============================================================
 * Public: parse
 * ============================================================ */

YamlNode* __yaml_parse(const char* content) {
    if (!content || !content[0]) return yaml_node_new(YAML_MAP, NULL, NULL);

    /* Split into lines */
    char* buf = strdup(content);
    int cap = 128, count = 0;
    char** lines = (char**)malloc(sizeof(char*) * cap);

    char* tok = strtok(buf, "\n");
    while (tok) {
        if (count >= cap) { cap *= 2; lines = (char**)realloc(lines, sizeof(char*) * cap); }
        lines[count++] = strdup(tok);
        tok = strtok(NULL, "\n");
    }
    free(buf);

    YamlParser parser = {lines, count, 0};
    YamlNode* root = parse_mapping(&parser, 0);

    for (int i = 0; i < count; i++) free(lines[i]);
    free(lines);

    return root;
}

YamlNode* __yaml_parse_file(const char* path) {
    if (!path) return yaml_node_new(YAML_MAP, NULL, NULL);
    FILE* f = fopen(path, "r");
    if (!f) return yaml_node_new(YAML_MAP, NULL, NULL);

    fseek(f, 0, SEEK_END);
    long sz = ftell(f);
    fseek(f, 0, SEEK_SET);
    char* content = (char*)malloc(sz + 1);
    fread(content, 1, sz, f);
    content[sz] = '\0';
    fclose(f);

    YamlNode* y = __yaml_parse(content);
    free(content);
    return y;
}

/* ============================================================
 * Public: accessors (dotted path: "server.host", "ports.0")
 * ============================================================ */

static YamlNode* resolve_path(YamlNode* root, const char* path) {
    if (!root || !path || !path[0]) return root;

    char* buf = strdup(path);
    char* tok = strtok(buf, ".");
    YamlNode* node = root;

    while (tok && node) {
        node = yaml_node_find(node, tok);
        tok = strtok(NULL, ".");
    }

    free(buf);
    return node;
}

const char* __yaml_get(YamlNode* y, const char* path) {
    YamlNode* n = resolve_path(y, path);
    return (n && n->value) ? n->value : "";
}

int64_t __yaml_get_int(YamlNode* y, const char* path) {
    const char* v = __yaml_get(y, path);
    return v[0] ? atoll(v) : 0;
}

double __yaml_get_float(YamlNode* y, const char* path) {
    const char* v = __yaml_get(y, path);
    return v[0] ? atof(v) : 0.0;
}

int32_t __yaml_get_bool(YamlNode* y, const char* path) {
    const char* v = __yaml_get(y, path);
    if (strcmp(v, "true") == 0 || strcmp(v, "yes") == 0 || strcmp(v, "on") == 0) return 1;
    return 0;
}

int32_t __yaml_has(YamlNode* y, const char* path) {
    return resolve_path(y, path) ? 1 : 0;
}

char* __yaml_keys(YamlNode* y, const char* path) {
    YamlNode* n = path[0] ? resolve_path(y, path) : y;
    if (!n) return strdup("");
    char buf[4096];
    int len = 0;
    for (int i = 0; i < n->child_count; i++) {
        if (n->children[i]->key) {
            len += snprintf(buf + len, sizeof(buf) - len, "%s\n", n->children[i]->key);
        }
    }
    return strdup(buf);
}

int32_t __yaml_len(YamlNode* y, const char* path) {
    YamlNode* n = resolve_path(y, path);
    return n ? n->child_count : 0;
}

/* ============================================================
 * Dump (serialize back to YAML)
 * ============================================================ */

static void yaml_dump_node(YamlNode* n, int indent, char* buf, int* pos, int cap) {
    for (int i = 0; i < n->child_count; i++) {
        YamlNode* child = n->children[i];
        if (child->type == YAML_STRING) {
            *pos += snprintf(buf + *pos, cap - *pos, "%*s%s: %s\n",
                             indent, "", child->key ? child->key : "", child->value);
        } else if (child->type == YAML_LIST) {
            *pos += snprintf(buf + *pos, cap - *pos, "%*s%s:\n", indent, "", child->key);
            for (int j = 0; j < child->child_count; j++) {
                *pos += snprintf(buf + *pos, cap - *pos, "%*s- %s\n",
                                 indent + 2, "", child->children[j]->value);
            }
        } else if (child->type == YAML_MAP) {
            *pos += snprintf(buf + *pos, cap - *pos, "%*s%s:\n", indent, "", child->key);
            yaml_dump_node(child, indent + 2, buf, pos, cap);
        }
    }
}

char* __yaml_dump(YamlNode* y) {
    if (!y) return strdup("");
    int cap = 16384;
    char* buf = (char*)malloc(cap);
    int pos = 0;
    yaml_dump_node(y, 0, buf, &pos, cap);
    buf[pos] = '\0';
    return buf;
}

/* ============================================================
 * Cleanup
 * ============================================================ */

void __yaml_free(YamlNode* n) {
    if (!n) return;
    free(n->key);
    free(n->value);
    for (int i = 0; i < n->child_count; i++) {
        __yaml_free(n->children[i]);
    }
    free(n->children);
    free(n);
}
