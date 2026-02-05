// desi_host.c - Persistent host process for hot reload
// Loads and reloads Desi shared library modules without process restart.

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdbool.h>
#include <signal.h>
#include <unistd.h>
#include <dlfcn.h>

// Module interface - every Desi .so exports these
typedef void (*InitFunc)(void);
typedef int (*MainFunc)(void);
typedef char* (*SaveFunc)(void);       // Returns JSON state
typedef void (*RestoreFunc)(char*);    // Receives JSON state

typedef struct {
    void* handle;
    char* path;
    InitFunc init;
    MainFunc main;
    SaveFunc save;
    RestoreFunc restore;
} DesiModule;

// Global state
static DesiModule* current_module = NULL;
static volatile sig_atomic_t reload_requested = 0;
static char* pending_module_path = NULL;

// Signal handler for reload requests
static void handle_reload_signal(int sig) {
    (void)sig;
    reload_requested = 1;
}

// Load a Desi module
static DesiModule* load_module(const char* path) {
    DesiModule* mod = (DesiModule*)calloc(1, sizeof(DesiModule));
    if (!mod) return NULL;

    mod->handle = dlopen(path, RTLD_NOW);
    if (!mod->handle) {
        fprintf(stderr, "[desi-host] dlopen failed: %s\n", dlerror());
        free(mod);
        return NULL;
    }

    mod->path = strdup(path);

    // Required exports
    mod->init = (InitFunc)dlsym(mod->handle, "__desi_module_init");
    mod->main = (MainFunc)dlsym(mod->handle, "__top__");

    // Optional state hooks
    mod->save = (SaveFunc)dlsym(mod->handle, "__on_reload_save__");
    mod->restore = (RestoreFunc)dlsym(mod->handle, "__on_reload_restore__");

    if (!mod->main) {
        fprintf(stderr, "[desi-host] Warning: module has no __top__ entry point\n");
    }

    return mod;
}

// Unload a module
static void unload_module(DesiModule* mod) {
    if (!mod) return;
    if (mod->handle) {
        dlclose(mod->handle);
    }
    free(mod->path);
    free(mod);
}

// Perform hot reload
static int do_reload(const char* new_path) {
    fprintf(stderr, "[desi-host] 🔄 Hot reloading: %s\n", new_path);

    // Step 1: Save state from current module
    char* saved_state = NULL;
    if (current_module && current_module->save) {
        fprintf(stderr, "[desi-host]   Saving state...\n");
        saved_state = current_module->save();
    }

    // Step 2: Load new module
    DesiModule* new_mod = load_module(new_path);
    if (!new_mod) {
        fprintf(stderr, "[desi-host] ❌ Failed to load new module\n");
        free(saved_state);
        return -1;
    }

    // Step 3: Unload old module
    if (current_module) {
        unload_module(current_module);
    }
    current_module = new_mod;

    // Step 4: Initialize new module
    if (current_module->init) {
        current_module->init();
    }

    // Step 5: Restore state
    if (saved_state && current_module->restore) {
        fprintf(stderr, "[desi-host]   Restoring state...\n");
        current_module->restore(saved_state);
        free(saved_state);
    }

    fprintf(stderr, "[desi-host] ✅ Reload complete\n");
    return 0;
}

// Check for reload request (call from main loop)
void __desi_check_reload(void) {
    if (reload_requested && pending_module_path) {
        reload_requested = 0;
        do_reload(pending_module_path);
        free(pending_module_path);
        pending_module_path = NULL;
    }
}

// Set pending reload path (called by watcher via IPC)
void __desi_queue_reload(const char* path) {
    if (pending_module_path) free(pending_module_path);
    pending_module_path = strdup(path);
    reload_requested = 1;
}

// Main host entry point
int desi_host_main(int argc, char** argv) {
    if (argc < 2) {
        fprintf(stderr, "Usage: desi-host <module.so>\n");
        return 1;
    }

    const char* initial_module = argv[1];

    // Set up signal handler for reload
    struct sigaction sa;
    sa.sa_handler = handle_reload_signal;
    sigemptyset(&sa.sa_mask);
    sa.sa_flags = 0;
    sigaction(SIGUSR1, &sa, NULL);

    fprintf(stderr, "[desi-host] 🚀 Starting with module: %s\n", initial_module);
    fprintf(stderr, "[desi-host]    Send SIGUSR1 to reload\n");
    fprintf(stderr, "[desi-host]    PID: %d\n", getpid());

    // Load initial module
    current_module = load_module(initial_module);
    if (!current_module) {
        return 1;
    }

    // Initialize
    if (current_module->init) {
        current_module->init();
    }

    // Run main
    int result = 0;
    if (current_module->main) {
        result = current_module->main();
    }

    // Cleanup
    unload_module(current_module);

    return result;
}

// Entry point when compiled as standalone host
#ifndef DESI_HOST_EMBEDDED
int main(int argc, char** argv) {
    return desi_host_main(argc, argv);
}
#endif
