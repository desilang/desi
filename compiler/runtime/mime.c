/*
 * mime.c — MIME type detection for Desi stdlib
 *
 * Maps file extensions to MIME types. Used by http_server for Content-Type.
 *
 * Public API:
 *   __mime_from_ext(ext)         → MIME type for extension (".json" → "application/json")
 *   __mime_from_path(path)      → MIME type from file path
 *   __mime_ext_for(mime_type)   → extension for MIME type ("text/html" → ".html")
 *   __mime_is_text(mime_type)   → 1 if text-based MIME type
 *   __mime_is_binary(mime_type) → 1 if binary MIME type
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <ctype.h>
#include <stdint.h>

typedef struct {
    const char* ext;
    const char* mime;
} MimeEntry;

/* Comprehensive MIME type table */
static const MimeEntry mime_table[] = {
    /* Text */
    {".html",  "text/html"},
    {".htm",   "text/html"},
    {".css",   "text/css"},
    {".js",    "text/javascript"},
    {".mjs",   "text/javascript"},
    {".json",  "application/json"},
    {".xml",   "application/xml"},
    {".txt",   "text/plain"},
    {".csv",   "text/csv"},
    {".md",    "text/markdown"},
    {".yaml",  "text/yaml"},
    {".yml",   "text/yaml"},
    {".toml",  "application/toml"},
    {".ini",   "text/plain"},
    {".cfg",   "text/plain"},
    {".log",   "text/plain"},
    {".rtf",   "application/rtf"},

    /* Images */
    {".png",   "image/png"},
    {".jpg",   "image/jpeg"},
    {".jpeg",  "image/jpeg"},
    {".gif",   "image/gif"},
    {".svg",   "image/svg+xml"},
    {".ico",   "image/x-icon"},
    {".webp",  "image/webp"},
    {".avif",  "image/avif"},
    {".bmp",   "image/bmp"},
    {".tiff",  "image/tiff"},
    {".tif",   "image/tiff"},

    /* Audio */
    {".mp3",   "audio/mpeg"},
    {".wav",   "audio/wav"},
    {".ogg",   "audio/ogg"},
    {".flac",  "audio/flac"},
    {".aac",   "audio/aac"},
    {".m4a",   "audio/mp4"},
    {".weba",  "audio/webm"},

    /* Video */
    {".mp4",   "video/mp4"},
    {".webm",  "video/webm"},
    {".avi",   "video/x-msvideo"},
    {".mov",   "video/quicktime"},
    {".mkv",   "video/x-matroska"},
    {".flv",   "video/x-flv"},
    {".wmv",   "video/x-ms-wmv"},
    {".m4v",   "video/mp4"},

    /* Fonts */
    {".woff",  "font/woff"},
    {".woff2", "font/woff2"},
    {".ttf",   "font/ttf"},
    {".otf",   "font/otf"},
    {".eot",   "application/vnd.ms-fontobject"},

    /* Archives */
    {".zip",   "application/zip"},
    {".gz",    "application/gzip"},
    {".tar",   "application/x-tar"},
    {".bz2",   "application/x-bzip2"},
    {".7z",    "application/x-7z-compressed"},
    {".rar",   "application/vnd.rar"},
    {".xz",    "application/x-xz"},

    /* Documents */
    {".pdf",   "application/pdf"},
    {".doc",   "application/msword"},
    {".docx",  "application/vnd.openxmlformats-officedocument.wordprocessingml.document"},
    {".xls",   "application/vnd.ms-excel"},
    {".xlsx",  "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"},
    {".ppt",   "application/vnd.ms-powerpoint"},
    {".pptx",  "application/vnd.openxmlformats-officedocument.presentationml.presentation"},

    /* Programming */
    {".wasm",  "application/wasm"},
    {".map",   "application/json"},
    {".ts",    "text/typescript"},
    {".tsx",   "text/tsx"},
    {".jsx",   "text/jsx"},
    {".py",    "text/x-python"},
    {".go",    "text/x-go"},
    {".rs",    "text/x-rust"},
    {".c",     "text/x-c"},
    {".h",     "text/x-c"},
    {".cpp",   "text/x-c++"},
    {".java",  "text/x-java"},
    {".rb",    "text/x-ruby"},
    {".php",   "text/x-php"},
    {".sh",    "text/x-shellscript"},
    {".sql",   "application/sql"},
    {".desi",  "text/x-desi"},

    /* Data */
    {".db",    "application/x-sqlite3"},
    {".sqlite","application/x-sqlite3"},

    /* Misc */
    {".rss",   "application/rss+xml"},
    {".atom",  "application/atom+xml"},
    {".manifest", "text/cache-manifest"},
    {".ics",   "text/calendar"},
    {".vcf",   "text/vcard"},

    {NULL, NULL}
};

/* ============================================================
 * Public API
 * ============================================================ */

const char* __mime_from_ext(const char* ext) {
    if (!ext || !ext[0]) return "application/octet-stream";

    /* Normalize: ensure leading dot, lowercase */
    char norm[32];
    int j = 0;
    if (ext[0] != '.') norm[j++] = '.';
    for (int i = 0; ext[i] && j < 30; i++) {
        norm[j++] = tolower((unsigned char)ext[i]);
    }
    norm[j] = '\0';

    for (int i = 0; mime_table[i].ext; i++) {
        if (strcmp(norm, mime_table[i].ext) == 0) {
            return mime_table[i].mime;
        }
    }
    return "application/octet-stream";
}

const char* __mime_from_path(const char* path) {
    if (!path) return "application/octet-stream";
    const char* dot = strrchr(path, '.');
    if (!dot) return "application/octet-stream";
    return __mime_from_ext(dot);
}

const char* __mime_ext_for(const char* mime_type) {
    if (!mime_type) return "";
    for (int i = 0; mime_table[i].ext; i++) {
        if (strcmp(mime_type, mime_table[i].mime) == 0) {
            return mime_table[i].ext;
        }
    }
    return "";
}

int32_t __mime_is_text(const char* mime_type) {
    if (!mime_type) return 0;
    if (strncmp(mime_type, "text/", 5) == 0) return 1;
    if (strcmp(mime_type, "application/json") == 0) return 1;
    if (strcmp(mime_type, "application/xml") == 0) return 1;
    if (strcmp(mime_type, "application/sql") == 0) return 1;
    if (strcmp(mime_type, "application/toml") == 0) return 1;
    if (strstr(mime_type, "+xml")) return 1;
    if (strstr(mime_type, "+json")) return 1;
    return 0;
}

int32_t __mime_is_binary(const char* mime_type) {
    return !__mime_is_text(mime_type);
}
