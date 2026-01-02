// Desi JSON Runtime - Fast JSON parser and serializer
// Part of hybrid C+Desi stdlib architecture

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <ctype.h>
#include <math.h>

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
    
    // Parse the number
    char* numstr = strndup(p->input + start, p->pos - start);
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

// Get boolean value
int __json_get_bool(JsonNode* node) {
    return (node && node->type == JSON_BOOL) ? node->bool_val : 0;
}

// Get number value
double __json_get_number(JsonNode* node) {
    return (node && node->type == JSON_NUMBER) ? node->num_val : 0.0;
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
