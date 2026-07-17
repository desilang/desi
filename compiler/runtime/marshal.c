#include <stdlib.h>
#include <string.h>
#include <stdio.h>
#include <stdint.h>
#include <stdbool.h>
#include <ctype.h>
#include "list.h"
#include "dict.h"

// Define DesiBytes struct to match bytes.c
typedef struct {
    unsigned char* data;
    size_t len;
} DesiBytes;

DesiBytes* __bytes_new(const unsigned char* data, int32_t len);

// DesiTypeInfo definition
typedef struct {
    uint64_t id;
    const char* name;
    size_t size;
} DesiTypeInfo;

// Marshal Type Representation for Parsing Type Names
typedef enum {
    MARSHAL_INT,
    MARSHAL_FLOAT,
    MARSHAL_BOOL,
    MARSHAL_STR,
    MARSHAL_NONE,
    MARSHAL_LIST,
    MARSHAL_DICT,
    MARSHAL_TUPLE
} MarshalKind;

typedef struct MarshalType {
    MarshalKind kind;
    struct MarshalType* key;
    struct MarshalType* val;
    struct MarshalType** elems;
    int elem_count;
} MarshalType;

static MarshalType* parse_type(const char** p) {
    while (**p == ' ' || **p == '\t') (*p)++;
    
    MarshalType* t = calloc(1, sizeof(MarshalType));
    if (strncmp(*p, "int", 3) == 0 && !isalnum((*p)[3]) && (*p)[3] != '_') {
        t->kind = MARSHAL_INT;
        *p += 3;
        return t;
    }
    if (strncmp(*p, "float", 5) == 0 && !isalnum((*p)[5]) && (*p)[5] != '_') {
        t->kind = MARSHAL_FLOAT;
        *p += 5;
        return t;
    }
    if (strncmp(*p, "bool", 4) == 0 && !isalnum((*p)[4]) && (*p)[4] != '_') {
        t->kind = MARSHAL_BOOL;
        *p += 4;
        return t;
    }
    if ((strncmp(*p, "str", 3) == 0 && !isalnum((*p)[3]) && (*p)[3] != '_') ||
        (strncmp(*p, "string", 6) == 0 && !isalnum((*p)[6]) && (*p)[6] != '_')) {
        t->kind = MARSHAL_STR;
        if (**p == 's') *p += 3; else *p += 6;
        return t;
    }
    if (strncmp(*p, "none", 4) == 0 && !isalnum((*p)[4]) && (*p)[4] != '_') {
        t->kind = MARSHAL_NONE;
        *p += 4;
        return t;
    }
    if (strncmp(*p, "list[", 5) == 0 || strncmp(*p, "list<", 5) == 0) {
        t->kind = MARSHAL_LIST;
        *p += 5;
        t->val = parse_type(p);
        if (**p == ']' || **p == '>') (*p)++;
        return t;
    }
    if (strncmp(*p, "dict[", 5) == 0 || strncmp(*p, "dict<", 5) == 0) {
        t->kind = MARSHAL_DICT;
        *p += 5;
        t->key = parse_type(p);
        while (**p == ' ' || **p == '\t' || **p == ',') (*p)++;
        t->val = parse_type(p);
        if (**p == ']' || **p == '>') (*p)++;
        return t;
    }
    if (strncmp(*p, "tuple[", 6) == 0 || strncmp(*p, "tuple<", 6) == 0) {
        t->kind = MARSHAL_TUPLE;
        *p += 6;
        int cap = 4;
        t->elems = malloc(cap * sizeof(MarshalType*));
        t->elem_count = 0;
        while (**p && **p != ']' && **p != '>') {
            if (t->elem_count >= cap) {
                cap *= 2;
                t->elems = realloc(t->elems, cap * sizeof(MarshalType*));
            }
            t->elems[t->elem_count++] = parse_type(p);
            while (**p == ' ' || **p == '\t') (*p)++;
            if (**p == ',') (*p)++;
        }
        if (**p == ']' || **p == '>') (*p)++;
        return t;
    }
    t->kind = MARSHAL_NONE;
    return t;
}

static void free_marshal_type(MarshalType* t) {
    if (!t) return;
    free_marshal_type(t->key);
    free_marshal_type(t->val);
    if (t->elems) {
        for (int i = 0; i < t->elem_count; i++) {
            free_marshal_type(t->elems[i]);
        }
        free(t->elems);
    }
    free(t);
}

// Write Buffer Implementation
typedef struct {
    unsigned char* data;
    size_t len;
    size_t cap;
} Buffer;

static void buf_init(Buffer* b) {
    b->cap = 128;
    b->len = 0;
    b->data = malloc(b->cap);
}

static void buf_write_byte(Buffer* b, unsigned char val) {
    if (b->len >= b->cap) {
        b->cap *= 2;
        b->data = realloc(b->data, b->cap);
    }
    b->data[b->len++] = val;
}

static void buf_write_bytes(Buffer* b, const unsigned char* src, size_t size) {
    while (b->len + size > b->cap) {
        b->cap *= 2;
        b->data = realloc(b->data, b->cap);
    }
    memcpy(b->data + b->len, src, size);
    b->len += size;
}

// Read Buffer Implementation
typedef struct {
    const unsigned char* data;
    size_t pos;
    size_t len;
} Reader;

static unsigned char read_byte(Reader* r) {
    if (r->pos >= r->len) return 0;
    return r->data[r->pos++];
}

static void read_bytes(Reader* r, unsigned char* dest, size_t size) {
    // Overflow-safe bounds check: r->pos + size could wrap for huge
    // stream-supplied sizes, so compare against the remaining span.
    if (size > r->len - r->pos) {
        memset(dest, 0, size);
        r->pos = r->len;
        return;
    }
    memcpy(dest, r->data + r->pos, size);
    r->pos += size;
}

// remaining returns how many unread bytes the reader holds.
static size_t reader_remaining(const Reader* r) {
    return r->len - r->pos;
}

// Recursive Serialization
static void serialize_val(Buffer* b, void* val, MarshalType* mt) {
    if (!mt) return;
    
    switch (mt->kind) {
        case MARSHAL_NONE:
            buf_write_byte(b, 0x08);
            break;
            
        case MARSHAL_INT: {
            buf_write_byte(b, 0x01);
            int32_t v = (val == NULL) ? 0 : *(int32_t*)val;
            buf_write_bytes(b, (unsigned char*)&v, 4);
            break;
        }
        
        case MARSHAL_FLOAT: {
            buf_write_byte(b, 0x02);
            double v = (val == NULL) ? 0.0 : *(double*)val;
            buf_write_bytes(b, (unsigned char*)&v, 8);
            break;
        }
        
        case MARSHAL_BOOL: {
            buf_write_byte(b, 0x03);
            bool v = (val == NULL) ? false : *(bool*)val;
            unsigned char byte_val = v ? 1 : 0;
            buf_write_byte(b, byte_val);
            break;
        }
        
        case MARSHAL_STR: {
            buf_write_byte(b, 0x04);
            const char* s = (const char*)val;
            uint64_t len = s ? strlen(s) : 0;
            buf_write_bytes(b, (unsigned char*)&len, 8);
            if (len > 0) {
                buf_write_bytes(b, (const unsigned char*)s, len);
            }
            break;
        }
        
        case MARSHAL_LIST: {
            buf_write_byte(b, 0x05);
            DesiList* l = (DesiList*)val;
            uint64_t len = l ? l->length : 0;
            buf_write_bytes(b, (unsigned char*)&len, 8);
            for (size_t i = 0; i < len; i++) {
                void* elem = l->data[i];
                if (mt->val->kind == MARSHAL_INT || mt->val->kind == MARSHAL_BOOL) {
                    serialize_val(b, &elem, mt->val);
                } else {
                    serialize_val(b, elem, mt->val);
                }
            }
            break;
        }
        
        case MARSHAL_DICT: {
            buf_write_byte(b, 0x06);
            dict_t* d = (dict_t*)val;
            uint64_t len = d ? d->entry_count : 0;
            buf_write_bytes(b, (unsigned char*)&len, 8);
            if (d) {
                for (size_t i = 0; i < d->bucket_count; i++) {
                    dict_entry_t* entry = d->buckets[i];
                    while (entry) {
                        // Key
                        if (mt->key->kind == MARSHAL_INT || mt->key->kind == MARSHAL_BOOL) {
                            serialize_val(b, &entry->key_int, mt->key);
                        } else if (mt->key->kind == MARSHAL_FLOAT) {
                            serialize_val(b, &entry->key_float, mt->key);
                        } else {
                            serialize_val(b, entry->key_str, mt->key);
                        }
                        
                        // Value
                        if (mt->val->kind == MARSHAL_INT || mt->val->kind == MARSHAL_FLOAT || mt->val->kind == MARSHAL_BOOL) {
                            serialize_val(b, entry->value, mt->val);
                        } else {
                            serialize_val(b, *(void**)entry->value, mt->val);
                        }
                        
                        entry = entry->next;
                    }
                }
            }
            break;
        }
        
        case MARSHAL_TUPLE: {
            buf_write_byte(b, 0x07);
            uint64_t len = mt->elem_count;
            buf_write_bytes(b, (unsigned char*)&len, 8);
            void** elements = (void**)val;
            for (int i = 0; i < mt->elem_count; i++) {
                void* elem = elements[i];
                serialize_val(b, elem, mt->elems[i]);
            }
            break;
        }
    }
}

// Expected wire tag for each MarshalKind (indexed by enum value).
static const unsigned char marshal_kind_tags[] = {
    0x01, // MARSHAL_INT
    0x02, // MARSHAL_FLOAT
    0x03, // MARSHAL_BOOL
    0x04, // MARSHAL_STR
    0x08, // MARSHAL_NONE
    0x05, // MARSHAL_LIST
    0x06, // MARSHAL_DICT
    0x07, // MARSHAL_TUPLE
};

// Recursive Deserialization
//
// Malformed input policy: this parses attacker-controllable bytes, so a
// wrong tag or a length that exceeds the remaining input poisons the
// reader (r->pos = r->len). Every subsequent read then zero-fills, and
// the result degrades to deterministic zero values / empty collections —
// never an out-of-bounds access, overflowing allocation, or crash.
static void* deserialize_val(Reader* r, MarshalType* mt) {
    if (!mt) return NULL;

    unsigned char tag = read_byte(r);
    if (tag != marshal_kind_tags[mt->kind]) {
        r->pos = r->len; // mismatched/corrupt stream — poison
    }

    switch (mt->kind) {
        case MARSHAL_NONE:
            return NULL;
            
        case MARSHAL_INT: {
            int32_t v32 = 0;
            read_bytes(r, (unsigned char*)&v32, 4);
            int64_t* box = calloc(1, 8);
            *(int32_t*)box = v32;
            return box;
        }
        
        case MARSHAL_FLOAT: {
            double v = 0.0;
            read_bytes(r, (unsigned char*)&v, 8);
            double* box = malloc(8);
            *box = v;
            return box;
        }
        
        case MARSHAL_BOOL: {
            unsigned char byte_val = read_byte(r);
            int64_t* box = calloc(1, 8);
            *(bool*)box = byte_val ? true : false;
            return box;
        }
        
        case MARSHAL_STR: {
            uint64_t len = 0;
            read_bytes(r, (unsigned char*)&len, 8);
            // A valid stream can't claim more content than it holds; a
            // clamped len also keeps malloc(len + 1) from overflowing.
            if (len > reader_remaining(r)) {
                len = 0;
                r->pos = r->len;
            }
            char* s = malloc((size_t)len + 1);
            if (!s) return strdup("");
            if (len > 0) read_bytes(r, (unsigned char*)s, (size_t)len);
            s[len] = '\0';
            return s;
        }
        
        case MARSHAL_LIST: {
            uint64_t len = 0;
            read_bytes(r, (unsigned char*)&len, 8);
            // Each element occupies at least one byte (its tag) — a larger
            // count is malformed and would loop allocating from an
            // exhausted reader.
            if (len > reader_remaining(r)) {
                len = 0;
                r->pos = r->len;
            }
            int tag = 3;
            if (mt->val->kind == MARSHAL_INT) tag = 0;
            else if (mt->val->kind == MARSHAL_STR) tag = 1;
            else if (mt->val->kind == MARSHAL_BOOL) tag = 2;
            DesiList* l = list_new(tag, NULL);
            for (uint64_t i = 0; i < len; i++) {
                void* elem = deserialize_val(r, mt->val);
                if (mt->val->kind == MARSHAL_INT || mt->val->kind == MARSHAL_BOOL) {
                    if (mt->val->kind == MARSHAL_INT) {
                        int32_t val = *(int32_t*)elem;
                        free(elem);
                        list_append(l, (void*)(intptr_t)val, tag);
                    } else {
                        bool val = *(bool*)elem;
                        free(elem);
                        list_append(l, (void*)(intptr_t)val, tag);
                    }
                } else {
                    list_append(l, elem, tag);
                }
            }
            return l;
        }
        
        case MARSHAL_DICT: {
            uint64_t len = 0;
            read_bytes(r, (unsigned char*)&len, 8);
            // Each entry needs at least two tag bytes — larger is malformed.
            if (len > reader_remaining(r)) {
                len = 0;
                r->pos = r->len;
            }
            int key_tag = 4;
            if (mt->key->kind == MARSHAL_INT) key_tag = 0;
            else if (mt->key->kind == MARSHAL_STR) key_tag = 1;
            else if (mt->key->kind == MARSHAL_BOOL) key_tag = 2;
            else if (mt->key->kind == MARSHAL_FLOAT) key_tag = 3;
            
            int val_tag = 4;
            if (mt->val->kind == MARSHAL_INT) val_tag = 0;
            else if (mt->val->kind == MARSHAL_STR) val_tag = 1;
            else if (mt->val->kind == MARSHAL_BOOL) val_tag = 2;
            else if (mt->val->kind == MARSHAL_FLOAT) val_tag = 3;
            
            size_t val_size = 8;
            dict_t* d = dict_new(key_tag, 8, val_size, val_tag, NULL, NULL, NULL);
            for (uint64_t i = 0; i < len; i++) {
                void* key = deserialize_val(r, mt->key);
                void* val = deserialize_val(r, mt->val);
                int64_t key_int = 0;
                const char* key_str = NULL;
                double key_float = 0.0;
                
                if (key_tag == 0) {
                    key_int = *(int32_t*)key;
                    free(key);
                } else if (key_tag == 2) {
                    key_int = *(bool*)key;
                    free(key);
                } else if (key_tag == 1) {
                    key_str = (const char*)key;
                } else if (key_tag == 3) {
                    key_float = *(double*)key;
                    free(key);
                }
                
                if (val_tag == 0 || val_tag == 2 || val_tag == 3) {
                    dict_insert(d, key_int, key_str, key_float, NULL, val, val_tag);
                    free(val);
                } else {
                    dict_insert(d, key_int, key_str, key_float, NULL, &val, val_tag);
                }
                if (key_str) free((void*)key_str);
            }
            return d;
        }
        
        case MARSHAL_TUPLE: {
            uint64_t len = 0;
            read_bytes(r, (unsigned char*)&len, 8);
            // The TYPE dictates the tuple arity; the stream length is only
            // validated. Looping to a stream-supplied len would index
            // mt->elems out of bounds.
            if (len != (uint64_t)mt->elem_count) {
                r->pos = r->len; // arity mismatch — poison, zero-fill elems
            }
            size_t n = (size_t)mt->elem_count;
            void** elements = malloc((n > 0 ? n : 1) * sizeof(void*));
            if (!elements) return NULL;
            for (size_t i = 0; i < n; i++) {
                elements[i] = deserialize_val(r, mt->elems[i]);
            }
            return elements;
        }
    }
    return NULL;
}

// Public API
DesiBytes* __marshal_dumps(void* val, DesiTypeInfo* type) {
    const char* name_p = type->name;
    MarshalType* mt = parse_type(&name_p);
    Buffer b;
    buf_init(&b);
    serialize_val(&b, val, mt);
    free_marshal_type(mt);
    
    DesiBytes* res = __bytes_new(b.data, (int32_t)b.len);
    free(b.data);
    return res;
}

void* __marshal_loads(DesiBytes* bytes, DesiTypeInfo* target_type) {
    if (!bytes) return NULL;
    const char* name_p = target_type->name;
    MarshalType* mt = parse_type(&name_p);
    Reader r = {
        .data = bytes->data,
        .pos = 0,
        .len = bytes->len
    };
    void* res = deserialize_val(&r, mt);
    free_marshal_type(mt);
    return res;
}
