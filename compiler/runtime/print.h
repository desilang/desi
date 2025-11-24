#ifndef PRINT_H
#define PRINT_H

#include <stdint.h>
#include <stdbool.h>

void print_int(int64_t value);
const char* bool_to_cstring(bool b);
void print_str(const char* s);

#endif
