// Desi JSON Runtime - Fast JSON parser and serializer
// Part of hybrid C+Desi stdlib architecture

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <ctype.h>
#include <math.h>
#include <stdint.h>

// ============================================================
// JSON Value Types
// ============================================================

typedef enum {
    JSON_NULL = 0,
    JSON_BOOL = 1,
    JSON_NUMBER = 2,
    JSON_STRING = 3,
    JSON_ARRAY = 4,
    JSON_OBJECT = 5
} JsonType;

typedef struct JsonNode {
    JsonType type;
    union {
        int bool_val;           // JSON_BOOL
        double num_val;         // JSON_NUMBER
        char* str_val;          // JSON_STRING (owned)
        struct {                // JSON_ARRAY
            struct JsonNode** items;
            int len;
            int cap;
        } array;
        struct {                // JSON_OBJECT
            char** keys;
            struct JsonNode** values;
            int len;
            int cap;
        } object;
    };
} JsonNode;

// ============================================================
// Parser State
// ============================================================

typedef struct {
    const char* input;
    size_t pos;
    size_t len;
    char* error;
} JsonParser;

// ============================================================
// Forward Declarations
// ============================================================

static JsonNode* parse_value(JsonParser* p);
static void skip_whitespace(JsonParser* p);

// ============================================================
// Memory Management
// ============================================================

static JsonNode* json_alloc(JsonType type) {
    JsonNode* node = (JsonNode*)calloc(1, sizeof(JsonNode));
    if (node) node->type = type;
    return node;
}

void __json_free(JsonNode* node) {
    if (!node) return;
    switch (node->type) {
        case JSON_STRING:
            free(node->str_val);
            break;
        case JSON_ARRAY:
            for (int i = 0; i < node->array.len; i++) {
                __json_free(node->array.items[i]);
            }
            free(node->array.items);
            break;
        case JSON_OBJECT:
            for (int i = 0; i < node->object.len; i++) {
                free(node->object.keys[i]);
                __json_free(node->object.values[i]);
            }
            free(node->object.keys);
            free(node->object.values);
            break;
        default:
            break;
    }
    free(node);
}

// ============================================================
// Parser Helpers
// ============================================================

static void skip_whitespace(JsonParser* p) {
    while (p->pos < p->len && isspace((unsigned char)p->input[p->pos])) {
        p->pos++;
    }
}

static int peek(JsonParser* p) {
    skip_whitespace(p);
    return p->pos < p->len ? p->input[p->pos] : 0;
}

static int consume(JsonParser* p) {
    skip_whitespace(p);
    return p->pos < p->len ? p->input[p->pos++] : 0;
}

static int match(JsonParser* p, const char* str) {
    skip_whitespace(p);
    size_t slen = strlen(str);
    if (p->pos + slen <= p->len && strncmp(p->input + p->pos, str, slen) == 0) {
        p->pos += slen;
        return 1;
    }
    return 0;
}

static void set_error(JsonParser* p, const char* msg) {
    if (!p->error) {
        p->error = strdup(msg);
    }
}

// ============================================================
// Parse String
// ============================================================

static char* parse_string_value(JsonParser* p) {
    if (consume(p) != '"') {
        set_error(p, "Expected '\"'");
        return NULL;
    }
    
    size_t start = p->pos;
    size_t cap = 64;
    char* result = (char*)malloc(cap);
    size_t len = 0;
    
    while (p->pos < p->len) {
        char c = p->input[p->pos++];
        if (c == '"') {
            result[len] = '\0';
            return result;
        }
        if (c == '\\' && p->pos < p->len) {
            char esc = p->input[p->pos++];
            switch (esc) {
                case '"':  c = '"'; break;
                case '\\': c = '\\'; break;
                case '/':  c = '/'; break;
                case 'b':  c = '\b'; break;
                case 'f':  c = '\f'; break;
                case 'n':  c = '\n'; break;
                case 'r':  c = '\r'; break;
                case 't':  c = '\t'; break;
                case 'u':
                    // Unicode escape - simplified, just skip 4 hex chars
                    p->pos += 4;
                    c = '?'; // Placeholder
                    break;
                default:
                    set_error(p, "Invalid escape sequence");
                    free(result);
                    return NULL;
            }
        }
        if (len + 1 >= cap) {
            cap *= 2;
            result = (char*)realloc(result, cap);
        }
        result[len++] = c;
    }
    
    set_error(p, "Unterminated string");
    free(result);
    return NULL;
}

// ============================================================
// Parse Number
// ============================================================

static JsonNode* parse_number(JsonParser* p) {
    skip_whitespace(p);
    size_t start = p->pos;
    
    // Optional minus
    if (p->pos < p->len && p->input[p->pos] == '-') p->pos++;
    
    // Integer part
    if (p->pos < p->len && p->input[p->pos] == '0') {
        p->pos++;
    } else {
        while (p->pos < p->len && isdigit((unsigned char)p->input[p->pos])) {
            p->pos++;
        }
    }
    
    // Fractional part
    if (p->pos < p->len && p->input[p->pos] == '.') {
        p->pos++;
        while (p->pos < p->len && isdigit((unsigned char)p->input[p->pos])) {
            p->pos++;
        }
    }
    
    // Exponent
    if (p->pos < p->len && (p->input[p->pos] == 'e' || p->input[p->pos] == 'E')) {
        p->pos++;
        if (p->pos < p->len && (p->input[p->pos] == '+' || p->input[p->pos] == '-')) {
            p->pos++;
        }
        while (p->pos < p->len && isdigit((unsigned char)p->input[p->pos])) {
            p->pos++;
        }
    }
    
    // Parse the number - use manual string copy since strndup is POSIX, not MSVC
    size_t numlen = p->pos - start;
    char* numstr = (char*)malloc(numlen + 1);
    memcpy(numstr, p->input + start, numlen);
    numstr[numlen] = '\0';
    double val = strtod(numstr, NULL);
    free(numstr);
    
    JsonNode* node = json_alloc(JSON_NUMBER);
    node->num_val = val;
    return node;
}

// ============================================================
// Parse Array
// ============================================================

static JsonNode* parse_array(JsonParser* p) {
    if (consume(p) != '[') {
        set_error(p, "Expected '['");
        return NULL;
    }
    
    JsonNode* node = json_alloc(JSON_ARRAY);
    node->array.cap = 8;
    node->array.items = (JsonNode**)malloc(node->array.cap * sizeof(JsonNode*));
    node->array.len = 0;
    
    if (peek(p) == ']') {
        consume(p);
        return node;
    }
    
    while (1) {
        JsonNode* item = parse_value(p);
        if (!item) {
            __json_free(node);
            return NULL;
        }
        
        if (node->array.len >= node->array.cap) {
            node->array.cap *= 2;
            node->array.items = (JsonNode**)realloc(node->array.items, 
                node->array.cap * sizeof(JsonNode*));
        }
        node->array.items[node->array.len++] = item;
        
        int c = peek(p);
        if (c == ']') {
            consume(p);
            return node;
        }
        if (c != ',') {
            set_error(p, "Expected ',' or ']'");
            __json_free(node);
            return NULL;
        }
        consume(p);
    }
}

// ============================================================
// Parse Object
// ============================================================

static JsonNode* parse_object(JsonParser* p) {
    if (consume(p) != '{') {
        set_error(p, "Expected '{'");
        return NULL;
    }
    
    JsonNode* node = json_alloc(JSON_OBJECT);
    node->object.cap = 8;
    node->object.keys = (char**)malloc(node->object.cap * sizeof(char*));
    node->object.values = (JsonNode**)malloc(node->object.cap * sizeof(JsonNode*));
    node->object.len = 0;
    
    if (peek(p) == '}') {
        consume(p);
        return node;
    }
    
    while (1) {
        // Parse key
        char* key = parse_string_value(p);
        if (!key) {
            __json_free(node);
            return NULL;
        }
        
        // Expect colon
        if (consume(p) != ':') {
            set_error(p, "Expected ':'");
            free(key);
            __json_free(node);
            return NULL;
        }
        
        // Parse value
        JsonNode* value = parse_value(p);
        if (!value) {
            free(key);
            __json_free(node);
            return NULL;
        }
        
        // Add to object
        if (node->object.len >= node->object.cap) {
            node->object.cap *= 2;
            node->object.keys = (char**)realloc(node->object.keys,
                node->object.cap * sizeof(char*));
            node->object.values = (JsonNode**)realloc(node->object.values,
                node->object.cap * sizeof(JsonNode*));
        }
        node->object.keys[node->object.len] = key;
        node->object.values[node->object.len] = value;
        node->object.len++;
        
        int c = peek(p);
        if (c == '}') {
            consume(p);
            return node;
        }
        if (c != ',') {
            set_error(p, "Expected ',' or '}'");
            __json_free(node);
            return NULL;
        }
        consume(p);
    }
}

// ============================================================
// Parse Value (main dispatcher)
// ============================================================

static JsonNode* parse_value(JsonParser* p) {
    skip_whitespace(p);
    
    if (p->pos >= p->len) {
        set_error(p, "Unexpected end of input");
        return NULL;
    }
    
    int c = p->input[p->pos];
    
    // String
    if (c == '"') {
        char* str = parse_string_value(p);
        if (!str) return NULL;
        JsonNode* node = json_alloc(JSON_STRING);
        node->str_val = str;
        return node;
    }
    
    // Number
    if (c == '-' || isdigit(c)) {
        return parse_number(p);
    }
    
    // Array
    if (c == '[') {
        return parse_array(p);
    }
    
    // Object
    if (c == '{') {
        return parse_object(p);
    }
    
    // true
    if (match(p, "true")) {
        JsonNode* node = json_alloc(JSON_BOOL);
        node->bool_val = 1;
        return node;
    }
    
    // false
    if (match(p, "false")) {
        JsonNode* node = json_alloc(JSON_BOOL);
        node->bool_val = 0;
        return node;
    }
    
    // null
    if (match(p, "null")) {
        return json_alloc(JSON_NULL);
    }
    
    set_error(p, "Unexpected character");
    return NULL;
}

// ============================================================
// Public API
// ============================================================

// Parse JSON string, returns NULL on error
JsonNode* __json_parse(const char* text) {
    if (!text) return NULL;
    
    JsonParser parser = {
        .input = text,
        .pos = 0,
        .len = strlen(text),
        .error = NULL
    };
    
    JsonNode* result = parse_value(&parser);
    
    if (parser.error) {
        free(parser.error);
        if (result) __json_free(result);
        return NULL;
    }
    
    // Check for trailing content
    skip_whitespace(&parser);
    if (parser.pos < parser.len) {
        __json_free(result);
        return NULL;
    }
    
    return result;
}

// Get JSON type
int __json_type(JsonNode* node) {
    return node ? node->type : JSON_NULL;
}

// Type check functions - return bool (0 or 1)
int __json_is_null(JsonNode* node) {
    return __json_type(node) == JSON_NULL;
}

int __json_is_bool(JsonNode* node) {
    return __json_type(node) == JSON_BOOL;
}

int __json_is_number(JsonNode* node) {
    return __json_type(node) == JSON_NUMBER;
}

int __json_is_string(JsonNode* node) {
    return __json_type(node) == JSON_STRING;
}

int __json_is_array(JsonNode* node) {
    return __json_type(node) == JSON_ARRAY;
}

int __json_is_object(JsonNode* node) {
    return __json_type(node) == JSON_OBJECT;
}

// Get boolean value
int __json_get_bool(JsonNode* node) {
    return (node && node->type == JSON_BOOL) ? node->bool_val : 0;
}

// Get number value (always returns double)
double __json_get_number(JsonNode* node) {
    return (node && node->type == JSON_NUMBER) ? node->num_val : 0.0;
}

// Check if number is a whole integer (Python-style smart detection)
int __json_is_int(JsonNode* node) {
    if (!node || node->type != JSON_NUMBER) return 0;
    double val = node->num_val;
    return val == (double)(int64_t)val;
}

// Get number as integer (Rust-style explicit accessor)
int __json_get_int(JsonNode* node) {
    if (!node || node->type != JSON_NUMBER) return 0;
    return (int)node->num_val;
}

// Rename get_number to get_float for clarity (alias)
double __json_get_float(JsonNode* node) {
    return __json_get_number(node);
}

// Get string value (borrowed pointer)
const char* __json_get_string(JsonNode* node) {
    return (node && node->type == JSON_STRING) ? node->str_val : "";
}

// Get array length
int __json_array_len(JsonNode* node) {
    return (node && node->type == JSON_ARRAY) ? node->array.len : 0;
}

// Get array item
JsonNode* __json_array_get(JsonNode* node, int index) {
    if (!node || node->type != JSON_ARRAY) return NULL;
    if (index < 0 || index >= node->array.len) return NULL;
    return node->array.items[index];
}

// Get object length
int __json_object_len(JsonNode* node) {
    return (node && node->type == JSON_OBJECT) ? node->object.len : 0;
}

// Get object key by index
const char* __json_object_key(JsonNode* node, int index) {
    if (!node || node->type != JSON_OBJECT) return NULL;
    if (index < 0 || index >= node->object.len) return NULL;
    return node->object.keys[index];
}

// Get object value by index
JsonNode* __json_object_value_at(JsonNode* node, int index) {
    if (!node || node->type != JSON_OBJECT) return NULL;
    if (index < 0 || index >= node->object.len) return NULL;
    return node->object.values[index];
}

// Get object value by key
JsonNode* __json_object_get(JsonNode* node, const char* key) {
    if (!node || node->type != JSON_OBJECT || !key) return NULL;
    for (int i = 0; i < node->object.len; i++) {
        if (strcmp(node->object.keys[i], key) == 0) {
            return node->object.values[i];
        }
    }
    return NULL;
}

// ============================================================
// JSON Builders (create nodes programmatically)
// ============================================================

JsonNode* __json_new_object(void) {
    JsonNode* node = json_alloc(JSON_OBJECT);
    node->object.cap = 8;
    node->object.keys = (char**)malloc(8 * sizeof(char*));
    node->object.values = (JsonNode**)malloc(8 * sizeof(JsonNode*));
    node->object.len = 0;
    return node;
}

JsonNode* __json_new_array(void) {
    JsonNode* node = json_alloc(JSON_ARRAY);
    node->array.cap = 8;
    node->array.items = (JsonNode**)malloc(8 * sizeof(JsonNode*));
    node->array.len = 0;
    return node;
}

JsonNode* __json_new_string(const char* str) {
    JsonNode* node = json_alloc(JSON_STRING);
    node->str_val = strdup(str ? str : "");
    return node;
}

JsonNode* __json_new_number(double val) {
    JsonNode* node = json_alloc(JSON_NUMBER);
    node->num_val = val;
    return node;
}

JsonNode* __json_new_bool(int val) {
    JsonNode* node = json_alloc(JSON_BOOL);
    node->bool_val = val ? 1 : 0;
    return node;
}

JsonNode* __json_new_null(void) {
    return json_alloc(JSON_NULL);
}

// Set a key-value pair on a JSON object (adds or updates)
void __json_object_set(JsonNode* obj, const char* key, JsonNode* val) {
    if (!obj || obj->type != JSON_OBJECT || !key) return;

    // Check if key already exists — update in place
    for (int i = 0; i < obj->object.len; i++) {
        if (strcmp(obj->object.keys[i], key) == 0) {
            __json_free(obj->object.values[i]);
            obj->object.values[i] = val;
            return;
        }
    }

    // New key — grow if needed
    if (obj->object.len >= obj->object.cap) {
        obj->object.cap *= 2;
        obj->object.keys = (char**)realloc(obj->object.keys,
            obj->object.cap * sizeof(char*));
        obj->object.values = (JsonNode**)realloc(obj->object.values,
            obj->object.cap * sizeof(JsonNode*));
    }
    obj->object.keys[obj->object.len] = strdup(key);
    obj->object.values[obj->object.len] = val;
    obj->object.len++;
}

// Push a value to a JSON array
void __json_array_push(JsonNode* arr, JsonNode* val) {
    if (!arr || arr->type != JSON_ARRAY) return;

    if (arr->array.len >= arr->array.cap) {
        arr->array.cap *= 2;
        arr->array.items = (JsonNode**)realloc(arr->array.items,
            arr->array.cap * sizeof(JsonNode*));
    }
    arr->array.items[arr->array.len++] = val;
}

// Remove a key from a JSON object
void __json_object_remove(JsonNode* obj, const char* key) {
    if (!obj || obj->type != JSON_OBJECT || !key) return;

    for (int i = 0; i < obj->object.len; i++) {
        if (strcmp(obj->object.keys[i], key) == 0) {
            free(obj->object.keys[i]);
            __json_free(obj->object.values[i]);
            // Shift remaining entries
            for (int j = i; j < obj->object.len - 1; j++) {
                obj->object.keys[j] = obj->object.keys[j + 1];
                obj->object.values[j] = obj->object.values[j + 1];
            }
            obj->object.len--;
            return;
        }
    }
}

// Get all keys of a JSON object as a JSON array of strings
JsonNode* __json_object_keys(JsonNode* obj) {
    JsonNode* arr = __json_new_array();
    if (!obj || obj->type != JSON_OBJECT) return arr;

    for (int i = 0; i < obj->object.len; i++) {
        __json_array_push(arr, __json_new_string(obj->object.keys[i]));
    }
    return arr;
}

// ============================================================
// Stringify
// ============================================================

static void stringify_to_buffer(JsonNode* node, char** buf, size_t* len, size_t* cap);

static void buf_append(char** buf, size_t* len, size_t* cap, const char* str) {
    size_t slen = strlen(str);
    while (*len + slen + 1 > *cap) {
        *cap *= 2;
        *buf = (char*)realloc(*buf, *cap);
    }
    memcpy(*buf + *len, str, slen);
    *len += slen;
    (*buf)[*len] = '\0';
}

static void buf_append_char(char** buf, size_t* len, size_t* cap, char c) {
    if (*len + 2 > *cap) {
        *cap *= 2;
        *buf = (char*)realloc(*buf, *cap);
    }
    (*buf)[(*len)++] = c;
    (*buf)[*len] = '\0';
}

static void stringify_string(const char* str, char** buf, size_t* len, size_t* cap) {
    buf_append_char(buf, len, cap, '"');
    for (const char* p = str; *p; p++) {
        switch (*p) {
            case '"':  buf_append(buf, len, cap, "\\\""); break;
            case '\\': buf_append(buf, len, cap, "\\\\"); break;
            case '\b': buf_append(buf, len, cap, "\\b"); break;
            case '\f': buf_append(buf, len, cap, "\\f"); break;
            case '\n': buf_append(buf, len, cap, "\\n"); break;
            case '\r': buf_append(buf, len, cap, "\\r"); break;
            case '\t': buf_append(buf, len, cap, "\\t"); break;
            default:
                if ((unsigned char)*p < 32) {
                    char esc[8];
                    snprintf(esc, sizeof(esc), "\\u%04x", (unsigned char)*p);
                    buf_append(buf, len, cap, esc);
                } else {
                    buf_append_char(buf, len, cap, *p);
                }
        }
    }
    buf_append_char(buf, len, cap, '"');
}

static void stringify_to_buffer(JsonNode* node, char** buf, size_t* len, size_t* cap) {
    if (!node) {
        buf_append(buf, len, cap, "null");
        return;
    }
    
    switch (node->type) {
        case JSON_NULL:
            buf_append(buf, len, cap, "null");
            break;
            
        case JSON_BOOL:
            buf_append(buf, len, cap, node->bool_val ? "true" : "false");
            break;
            
        case JSON_NUMBER: {
            char numstr[64];
            snprintf(numstr, sizeof(numstr), "%g", node->num_val);
            buf_append(buf, len, cap, numstr);
            break;
        }
            
        case JSON_STRING:
            stringify_string(node->str_val, buf, len, cap);
            break;
            
        case JSON_ARRAY:
            buf_append_char(buf, len, cap, '[');
            for (int i = 0; i < node->array.len; i++) {
                if (i > 0) buf_append_char(buf, len, cap, ',');
                stringify_to_buffer(node->array.items[i], buf, len, cap);
            }
            buf_append_char(buf, len, cap, ']');
            break;
            
        case JSON_OBJECT:
            buf_append_char(buf, len, cap, '{');
            for (int i = 0; i < node->object.len; i++) {
                if (i > 0) buf_append_char(buf, len, cap, ',');
                stringify_string(node->object.keys[i], buf, len, cap);
                buf_append_char(buf, len, cap, ':');
                stringify_to_buffer(node->object.values[i], buf, len, cap);
            }
            buf_append_char(buf, len, cap, '}');
            break;
    }
}

char* __json_stringify(JsonNode* node) {
    size_t cap = 256;
    size_t len = 0;
    char* buf = (char*)malloc(cap);
    buf[0] = '\0';
    
    stringify_to_buffer(node, &buf, &len, &cap);
    
    return buf;
}

// ============================================================
// Pretty-print Stringify (indented JSON)
// ============================================================

static void buf_append_indent(char** buf, size_t* len, size_t* cap, int depth, int indent) {
    int spaces = depth * indent;
    for (int i = 0; i < spaces; i++) {
        buf_append_char(buf, len, cap, ' ');
    }
}

static void stringify_pretty_to_buffer(JsonNode* node, char** buf, size_t* len, size_t* cap, int depth, int indent) {
    if (!node) {
        buf_append(buf, len, cap, "null");
        return;
    }

    switch (node->type) {
        case JSON_NULL:
            buf_append(buf, len, cap, "null");
            break;

        case JSON_BOOL:
            buf_append(buf, len, cap, node->bool_val ? "true" : "false");
            break;

        case JSON_NUMBER: {
            char numstr[64];
            // Use integer format for whole numbers
            if (node->num_val == (double)(int64_t)node->num_val &&
                node->num_val >= -1e15 && node->num_val <= 1e15) {
                snprintf(numstr, sizeof(numstr), "%lld", (long long)(int64_t)node->num_val);
            } else {
                snprintf(numstr, sizeof(numstr), "%g", node->num_val);
            }
            buf_append(buf, len, cap, numstr);
            break;
        }

        case JSON_STRING:
            stringify_string(node->str_val, buf, len, cap);
            break;

        case JSON_ARRAY:
            if (node->array.len == 0) {
                buf_append(buf, len, cap, "[]");
            } else {
                buf_append(buf, len, cap, "[\n");
                for (int i = 0; i < node->array.len; i++) {
                    buf_append_indent(buf, len, cap, depth + 1, indent);
                    stringify_pretty_to_buffer(node->array.items[i], buf, len, cap, depth + 1, indent);
                    if (i < node->array.len - 1) buf_append_char(buf, len, cap, ',');
                    buf_append_char(buf, len, cap, '\n');
                }
                buf_append_indent(buf, len, cap, depth, indent);
                buf_append_char(buf, len, cap, ']');
            }
            break;

        case JSON_OBJECT:
            if (node->object.len == 0) {
                buf_append(buf, len, cap, "{}");
            } else {
                buf_append(buf, len, cap, "{\n");
                for (int i = 0; i < node->object.len; i++) {
                    buf_append_indent(buf, len, cap, depth + 1, indent);
                    stringify_string(node->object.keys[i], buf, len, cap);
                    buf_append(buf, len, cap, ": ");
                    stringify_pretty_to_buffer(node->object.values[i], buf, len, cap, depth + 1, indent);
                    if (i < node->object.len - 1) buf_append_char(buf, len, cap, ',');
                    buf_append_char(buf, len, cap, '\n');
                }
                buf_append_indent(buf, len, cap, depth, indent);
                buf_append_char(buf, len, cap, '}');
            }
            break;
    }
}

// Pretty-print JSON with indentation (default 2 spaces)
// Python: json.dumps(data, indent=2)
char* __json_stringify_pretty(JsonNode* node, int indent) {
    if (indent <= 0) indent = 2;
    size_t cap = 512;
    size_t len = 0;
    char* buf = (char*)malloc(cap);
    buf[0] = '\0';

    stringify_pretty_to_buffer(node, &buf, &len, &cap, 0, indent);

    return buf;
}

// ============================================================
// Deep copy a JSON node
// ============================================================

JsonNode* __json_clone(JsonNode* node) {
    if (!node) return NULL;
    JsonNode* copy = json_alloc(node->type);
    switch (node->type) {
        case JSON_BOOL:
            copy->bool_val = node->bool_val;
            break;
        case JSON_NUMBER:
            copy->num_val = node->num_val;
            break;
        case JSON_STRING:
            copy->str_val = strdup(node->str_val ? node->str_val : "");
            break;
        case JSON_ARRAY:
            copy->array.cap = node->array.len > 0 ? node->array.len : 1;
            copy->array.items = (JsonNode**)malloc(copy->array.cap * sizeof(JsonNode*));
            copy->array.len = node->array.len;
            for (int i = 0; i < node->array.len; i++) {
                copy->array.items[i] = __json_clone(node->array.items[i]);
            }
            break;
        case JSON_OBJECT:
            copy->object.cap = node->object.len > 0 ? node->object.len : 1;
            copy->object.keys = (char**)malloc(copy->object.cap * sizeof(char*));
            copy->object.values = (JsonNode**)malloc(copy->object.cap * sizeof(JsonNode*));
            copy->object.len = node->object.len;
            for (int i = 0; i < node->object.len; i++) {
                copy->object.keys[i] = strdup(node->object.keys[i]);
                copy->object.values[i] = __json_clone(node->object.values[i]);
            }
            break;
        default:
            break;
    }
    return copy;
}

// ============================================================
// Merge two JSON objects (shallow merge, second wins on conflict)
// ============================================================

JsonNode* __json_merge(JsonNode* base, JsonNode* overlay) {
    if (!base || base->type != JSON_OBJECT) return __json_clone(overlay);
    if (!overlay || overlay->type != JSON_OBJECT) return __json_clone(base);

    JsonNode* result = __json_clone(base);
    for (int i = 0; i < overlay->object.len; i++) {
        // Check if key exists in result — update; else add
        int found = 0;
        for (int j = 0; j < result->object.len; j++) {
            if (strcmp(result->object.keys[j], overlay->object.keys[i]) == 0) {
                __json_free(result->object.values[j]);
                result->object.values[j] = __json_clone(overlay->object.values[i]);
                found = 1;
                break;
            }
        }
        if (!found) {
            __json_object_set(result, overlay->object.keys[i],
                              __json_clone(overlay->object.values[i]));
        }
    }
    return result;
}

// ============================================================
// Check if two JSON nodes are equal (deep equality)
// ============================================================

int __json_equals(JsonNode* a, JsonNode* b) {
    if (a == b) return 1;
    if (!a || !b) return 0;
    if (a->type != b->type) return 0;

    switch (a->type) {
        case JSON_NULL: return 1;
        case JSON_BOOL: return a->bool_val == b->bool_val;
        case JSON_NUMBER: return a->num_val == b->num_val;
        case JSON_STRING: return strcmp(a->str_val, b->str_val) == 0;
        case JSON_ARRAY:
            if (a->array.len != b->array.len) return 0;
            for (int i = 0; i < a->array.len; i++) {
                if (!__json_equals(a->array.items[i], b->array.items[i])) return 0;
            }
            return 1;
        case JSON_OBJECT:
            if (a->object.len != b->object.len) return 0;
            for (int i = 0; i < a->object.len; i++) {
                JsonNode* bval = __json_object_get(b, a->object.keys[i]);
                if (!bval || !__json_equals(a->object.values[i], bval)) return 0;
            }
            return 1;
    }
    return 0;
}

// ============================================================
// Check if a JSON object contains a key
// ============================================================

int __json_has_key(JsonNode* obj, const char* key) {
    if (!obj || obj->type != JSON_OBJECT || !key) return 0;
    for (int i = 0; i < obj->object.len; i++) {
        if (strcmp(obj->object.keys[i], key) == 0) return 1;
    }
    return 0;
}

// ============================================================
// Get all values of a JSON object as a JSON array
// ============================================================

JsonNode* __json_object_values(JsonNode* obj) {
    JsonNode* arr = __json_new_array();
    if (!obj || obj->type != JSON_OBJECT) return arr;
    for (int i = 0; i < obj->object.len; i++) {
        __json_array_push(arr, __json_clone(obj->object.values[i]));
    }
    return arr;
}

/* ---- Dict-to-JSON bridge ---- */
/* Converts a Desi runtime dict_t* directly to a JSON string. */
/* This avoids the JsonNode intermediate representation.       */

#include "dict.h"

static void json_buf_grow(char** buf, size_t* cap, size_t needed) {
    while (*cap < needed) {
        *cap *= 2;
        *buf = (char*)realloc(*buf, *cap);
    }
}

char* __dict_to_json_str(void* raw) {
    if (!raw) return strdup("null");
    dict_t* d = (dict_t*)raw;

    size_t cap = 256, pos = 0;
    char* buf = (char*)malloc(cap);
    buf[pos++] = '{';

    int first = 1;
    for (size_t i = 0; i < d->bucket_count; i++) {
        dict_entry_t* entry = d->buckets[i];
        while (entry) {
            /* comma separator */
            if (!first) {
                json_buf_grow(&buf, &cap, pos + 2);
                buf[pos++] = ',';
            }
            first = 0;

            /* === Key (always JSON-quoted) === */
            char key_tmp[128];
            switch (d->key_type_tag) {
                case TYPE_TAG_STR:
                    snprintf(key_tmp, sizeof(key_tmp), "%s",
                             entry->key_str ? entry->key_str : "");
                    break;
                case TYPE_TAG_INT:
                    snprintf(key_tmp, sizeof(key_tmp), "%lld",
                             (long long)entry->key_int);
                    break;
                case TYPE_TAG_BOOL:
                    snprintf(key_tmp, sizeof(key_tmp), "%s",
                             entry->key_int ? "true" : "false");
                    break;
                case TYPE_TAG_FLOAT:
                    snprintf(key_tmp, sizeof(key_tmp), "%g",
                             entry->key_float);
                    break;
                default:
                    snprintf(key_tmp, sizeof(key_tmp), "unknown");
            }
            size_t klen = strlen(key_tmp);
            json_buf_grow(&buf, &cap, pos + klen + 4);
            buf[pos++] = '"';
            memcpy(buf + pos, key_tmp, klen);
            pos += klen;
            buf[pos++] = '"';
            buf[pos++] = ':';

            /* === Value (type-aware, per-entry) === */
            char val_tmp[256];
            int vtt = entry->value_type_tag;

            /* For Any-typed dicts, the value_type_tag may be 0 (int).
               We try to detect the actual type from the value_size and
               the first few bytes. This is heuristic but covers the
               common case of dict[str, Any]. */
            if (d->value_to_str_fn != NULL) {
                /* Custom to-string: treat as quoted string */
                char* s = d->value_to_str_fn(entry->value);
                size_t slen = s ? strlen(s) : 4;
                json_buf_grow(&buf, &cap, pos + slen + 3);
                if (s) {
                    buf[pos++] = '"';
                    memcpy(buf + pos, s, slen);
                    pos += slen;
                    buf[pos++] = '"';
                } else {
                    memcpy(buf + pos, "null", 4);
                    pos += 4;
                }
            } else if (vtt == TYPE_TAG_STR) {
                char* str_val = *(char**)entry->value;
                if (!str_val) str_val = "";
                size_t slen = strlen(str_val);
                json_buf_grow(&buf, &cap, pos + slen + 3);
                buf[pos++] = '"';
                memcpy(buf + pos, str_val, slen);
                pos += slen;
                buf[pos++] = '"';
            } else if (vtt == TYPE_TAG_BOOL) {
                int64_t bv = 0;
                if (d->value_size >= sizeof(int64_t))
                    memcpy(&bv, entry->value, sizeof(int64_t));
                const char* bs = bv ? "true" : "false";
                size_t blen = strlen(bs);
                json_buf_grow(&buf, &cap, pos + blen + 1);
                memcpy(buf + pos, bs, blen);
                pos += blen;
            } else if (vtt == TYPE_TAG_FLOAT) {
                double fv = 0.0;
                if (d->value_size >= sizeof(double))
                    memcpy(&fv, entry->value, sizeof(double));
                int n = snprintf(val_tmp, sizeof(val_tmp), "%g", fv);
                json_buf_grow(&buf, &cap, pos + n + 1);
                memcpy(buf + pos, val_tmp, n);
                pos += n;
            } else {
                /* Default: integer */
                int64_t iv = 0;
                if (d->value_size >= sizeof(int64_t))
                    memcpy(&iv, entry->value, sizeof(int64_t));
                int n = snprintf(val_tmp, sizeof(val_tmp), "%lld", (long long)iv);
                json_buf_grow(&buf, &cap, pos + n + 1);
                memcpy(buf + pos, val_tmp, n);
                pos += n;
            }

            entry = entry->next;
        }
    }

    json_buf_grow(&buf, &cap, pos + 2);
    buf[pos++] = '}';
    buf[pos] = '\0';
    return buf;
}
