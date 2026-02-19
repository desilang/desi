// uuid.c - UUID generation module
// Uses /dev/urandom for v4 UUIDs. No external deps.
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <fcntl.h>
#include <ctype.h>

// Generate UUID v4 (random)
// Format: xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx
char* __uuid_v4(void) {
    unsigned char bytes[16];
    int fd = open("/dev/urandom", O_RDONLY);
    if (fd >= 0) {
        if (read(fd, bytes, 16) != 16) {
            close(fd);
            return strdup("00000000-0000-4000-8000-000000000000");
        }
        close(fd);
    } else {
        return strdup("00000000-0000-4000-8000-000000000000");
    }

    // Set version (4) and variant (RFC 4122)
    bytes[6] = (bytes[6] & 0x0F) | 0x40; // Version 4
    bytes[8] = (bytes[8] & 0x3F) | 0x80; // Variant 1

    char* uuid = (char*)malloc(37);
    if (!uuid) return strdup("");
    sprintf(uuid,
        "%02x%02x%02x%02x-%02x%02x-%02x%02x-%02x%02x-%02x%02x%02x%02x%02x%02x",
        bytes[0], bytes[1], bytes[2], bytes[3],
        bytes[4], bytes[5],
        bytes[6], bytes[7],
        bytes[8], bytes[9],
        bytes[10], bytes[11], bytes[12], bytes[13], bytes[14], bytes[15]);
    return uuid;
}

// Check if string is a valid UUID format
int __uuid_is_valid(const char* s) {
    if (!s) return 0;
    if (strlen(s) != 36) return 0;
    // Format: 8-4-4-4-12
    for (int i = 0; i < 36; i++) {
        if (i == 8 || i == 13 || i == 18 || i == 23) {
            if (s[i] != '-') return 0;
        } else {
            if (!isxdigit((unsigned char)s[i])) return 0;
        }
    }
    return 1;
}

// Return nil UUID
char* __uuid_nil(void) {
    return strdup("00000000-0000-0000-0000-000000000000");
}
