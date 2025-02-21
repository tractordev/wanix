#define _GNU_SOURCE
#include <dlfcn.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <pthread.h>
#include <stdbool.h>
#include <unistd.h>

// Add external declaration for environ
extern char **environ;

// Function pointer types for the real exec functions
typedef int (*execve_type)(const char*, char *const[], char *const[]);
typedef int (*execvp_type)(const char*, char *const[]);
typedef int (*execvpe_type)(const char*, char *const[], char *const[]);

// Global variables to store original function pointers
static execve_type real_execve = NULL;
static execvp_type real_execvp = NULL;
static execvpe_type real_execvpe = NULL;

// Mutex for thread safety
static pthread_mutex_t mutex = PTHREAD_MUTEX_INITIALIZER;
static pthread_once_t init_once = PTHREAD_ONCE_INIT;

// Initialize the original function pointers
static void init(void) {
    real_execve = (execve_type)dlsym(RTLD_NEXT, "execve");
    real_execvp = (execvp_type)dlsym(RTLD_NEXT, "execvp");
    real_execvpe = (execvpe_type)dlsym(RTLD_NEXT, "execvpe");
}

// Helper function to count arguments
static size_t count_args(char *const argv[]) {
    size_t count = 0;
    while (argv[count] != NULL) {
        count++;
    }
    return count;
}

// Common handling function for all exec variants
static int handle_execution(const char *path, char *const argv[], char *const envp[]) {
    if (path == NULL || argv == NULL) {
        return -1;
    }

    // Check if the path ends with .wasm
    size_t path_len = strlen(path);
    bool is_wasm = (path_len > 5 && strcmp(path + path_len - 5, ".wasm") == 0);

    if (is_wasm) {
        const char *wexec_path = "/bin/wexec";
        size_t arg_count = count_args(argv);
        
        // Allocate new argument array: wexec + wasm_path + original_args (minus argv[0]) + NULL
        char **new_argv = malloc(sizeof(char*) * (arg_count + 2));
        if (!new_argv) {
            return -1;
        }

        // Set up new argument array
        new_argv[0] = (char*)wexec_path;
        new_argv[1] = (char*)path;
        
        // Copy remaining arguments, skipping argv[0]
        for (size_t i = 1; i < arg_count; i++) {
            new_argv[i + 1] = argv[i];
        }
        new_argv[arg_count + 1] = NULL;

        // Execute wexec with the new arguments
        int result = real_execve(wexec_path, new_argv, envp);
        free(new_argv);
        return result;
    } else {
        // Normal execution
        return real_execve(path, argv, envp);
    }
}

// Exported functions that replace the standard exec functions

__attribute__((visibility("default")))
int execve(const char *path, char *const argv[], char *const envp[]) {
    pthread_once(&init_once, init);
    if (!real_execve) {
        return -1;
    }
    return handle_execution(path, argv, envp);
}

__attribute__((visibility("default")))
int execvp(const char *path, char *const argv[]) {
    pthread_once(&init_once, init);
    if (!real_execvp) {
        return -1;
    }
    return handle_execution(path, argv, environ);
}

__attribute__((visibility("default")))
int execvpe(const char *path, char *const argv[], char *const envp[]) {
    pthread_once(&init_once, init);
    if (!real_execvpe) {
        return -1;
    }
    return handle_execution(path, argv, envp);
}