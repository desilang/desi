// random.c - Random number generation module
// Uses /dev/urandom for secure randomness.
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>
#include <time.h>
#include <unistd.h>
#include <fcntl.h>
#include "list.h"

static int seeded = 0;

static void ensure_seeded(void) {
    if (!seeded) {
        unsigned int seed;
        int fd = open("/dev/urandom", O_RDONLY);
        if (fd >= 0 && read(fd, &seed, sizeof(seed)) == sizeof(seed)) {
            srand(seed);
            close(fd);
        } else {
            srand((unsigned int)(time(NULL) ^ getpid()));
        }
        seeded = 1;
    }
}

// ============================================================
// Core random functions
// ============================================================

void __random_seed(int n) {
    srand((unsigned int)n);
    seeded = 1;
}

int __random_randint(int a, int b) {
    ensure_seeded();
    if (a > b) { int tmp = a; a = b; b = tmp; }
    if (a == b) return a;
    long range = (long)b - (long)a + 1;
    return a + (int)(rand() % range);
}

double __random_random(void) {
    ensure_seeded();
    return (double)rand() / ((double)RAND_MAX + 1.0);
}

double __random_uniform(double a, double b) {
    ensure_seeded();
    double t = (double)rand() / (double)RAND_MAX;
    return a + t * (b - a);
}

// ============================================================
// List-based functions
// ============================================================

char* __random_choice(DesiList* items) {
    if (!items) return strdup("");
    int64_t len = list_len(items);
    if (len == 0) return strdup("");
    ensure_seeded();
    int64_t idx = (int64_t)(rand() % (int)len);
    const char* s = (const char*)list_get(items, idx);
    return s ? strdup(s) : strdup("");
}

DesiList* __random_shuffle(DesiList* items) {
    DesiList* result = list_new(1, NULL); // type_tag 1 = str
    if (!items) return result;
    int64_t len = list_len(items);
    if (len == 0) return result;
    ensure_seeded();

    int n = (int)len;
    int* indices = (int*)malloc(n * sizeof(int));
    if (!indices) return result;
    for (int i = 0; i < n; i++) indices[i] = i;

    // Fisher-Yates shuffle
    for (int i = n - 1; i > 0; i--) {
        int j = rand() % (i + 1);
        int tmp = indices[i];
        indices[i] = indices[j];
        indices[j] = tmp;
    }

    for (int i = 0; i < n; i++) {
        const char* s = (const char*)list_get(items, (int64_t)indices[i]);
        list_append(result, (void*)strdup(s ? s : ""), 1);
    }
    free(indices);
    return result;
}

DesiList* __random_sample(DesiList* items, int k) {
    DesiList* result = list_new(1, NULL);
    if (!items || k <= 0) return result;
    int64_t len = list_len(items);
    if (len == 0) return result;
    int n = (int)len;
    if (k > n) k = n;
    ensure_seeded();

    int* indices = (int*)malloc(n * sizeof(int));
    if (!indices) return result;
    for (int i = 0; i < n; i++) indices[i] = i;

    // Partial Fisher-Yates for first k elements
    for (int i = 0; i < k; i++) {
        int j = i + (rand() % (n - i));
        int tmp = indices[i];
        indices[i] = indices[j];
        indices[j] = tmp;
    }

    for (int i = 0; i < k; i++) {
        const char* s = (const char*)list_get(items, (int64_t)indices[i]);
        list_append(result, (void*)strdup(s ? s : ""), 1);
    }
    free(indices);
    return result;
}

// ============================================================
// Int-list overloads (type_tag 0 = int, stored as (void*)(intptr_t))
// ============================================================

int __random_choice_int(DesiList* items) {
    if (!items) return 0;
    int64_t len = list_len(items);
    if (len == 0) return 0;
    ensure_seeded();
    int64_t idx = (int64_t)(rand() % (int)len);
    return (int)(intptr_t)list_get(items, idx);
}

DesiList* __random_shuffle_int(DesiList* items) {
    DesiList* result = list_new(0, NULL); // type_tag 0 = int
    if (!items) return result;
    int64_t len = list_len(items);
    if (len == 0) return result;
    ensure_seeded();

    int n = (int)len;
    int* indices = (int*)malloc(n * sizeof(int));
    if (!indices) return result;
    for (int i = 0; i < n; i++) indices[i] = i;

    for (int i = n - 1; i > 0; i--) {
        int j = rand() % (i + 1);
        int tmp = indices[i];
        indices[i] = indices[j];
        indices[j] = tmp;
    }

    for (int i = 0; i < n; i++) {
        intptr_t val = (intptr_t)list_get(items, (int64_t)indices[i]);
        list_append(result, (void*)val, 0);
    }
    free(indices);
    return result;
}

DesiList* __random_sample_int(DesiList* items, int k) {
    DesiList* result = list_new(0, NULL);
    if (!items || k <= 0) return result;
    int64_t len = list_len(items);
    if (len == 0) return result;
    int n = (int)len;
    if (k > n) k = n;
    ensure_seeded();

    int* indices = (int*)malloc(n * sizeof(int));
    if (!indices) return result;
    for (int i = 0; i < n; i++) indices[i] = i;

    for (int i = 0; i < k; i++) {
        int j = i + (rand() % (n - i));
        int tmp = indices[i];
        indices[i] = indices[j];
        indices[j] = tmp;
    }

    for (int i = 0; i < k; i++) {
        intptr_t val = (intptr_t)list_get(items, (int64_t)indices[i]);
        list_append(result, (void*)val, 0);
    }
    free(indices);
    return result;
}

// ============================================================
// Secure/hex random
// ============================================================

char* __random_hex(int n) {
    if (n <= 0) return strdup("");
    unsigned char* buf = (unsigned char*)malloc(n);
    if (!buf) return strdup("");

    int fd = open("/dev/urandom", O_RDONLY);
    if (fd >= 0) {
        ssize_t got = read(fd, buf, n);
        close(fd);
        if (got != n) {
            ensure_seeded();
            for (int i = 0; i < n; i++) buf[i] = (unsigned char)(rand() & 0xFF);
        }
    } else {
        ensure_seeded();
        for (int i = 0; i < n; i++) buf[i] = (unsigned char)(rand() & 0xFF);
    }

    char* hex = (char*)malloc(2 * n + 1);
    if (!hex) { free(buf); return strdup(""); }
    for (int i = 0; i < n; i++) {
        sprintf(hex + 2 * i, "%02x", buf[i]);
    }
    hex[2 * n] = '\0';
    free(buf);
    return hex;
}

int __random_crypto_random(void) {
    int val = 0;
    int fd = open("/dev/urandom", O_RDONLY);
    if (fd >= 0) {
        if (read(fd, &val, sizeof(val)) != sizeof(val)) {
            val = rand();
        }
        close(fd);
    } else {
        ensure_seeded();
        val = rand();
    }
    return val < 0 ? -val : val;
}
