# Makefile for Desi Language
# Supports cross-platform builds via GOOS/GOARCH and CC/AR overrides

# Variables
CC ?= clang
AR ?= ar
GO ?= go
BUILD_DIR = build
BIN_DIR = bin
RUNTIME_SRC = compiler/runtime
RUNTIME_DB  = compiler/runtime/db
DECIMAL_SRC = compiler/runtime/decimal
DECIMAL_LIB = $(DECIMAL_SRC)/lib/libmpdec.a

# macOS deployment target detection
UNAME_S := $(shell uname -s)
ifeq ($(UNAME_S),Darwin)
  MACOSX_VERSION ?= $(shell sw_vers -productVersion 2>/dev/null | cut -d. -f1-2 || echo "12.0")
  export MACOSX_DEPLOYMENT_TARGET = $(MACOSX_VERSION)
  CFLAGS += -mmacosx-version-min=$(MACOSX_VERSION)
endif

# OpenSSL detection for HTTPS support (Homebrew or system)
OPENSSL_PREFIX := $(shell brew --prefix openssl 2>/dev/null || echo "")
ifeq ($(OPENSSL_PREFIX),)
  # Try standard system paths
  ifneq ($(wildcard /usr/include/openssl/ssl.h),)
    OPENSSL_CFLAGS =
    OPENSSL_LDFLAGS = -lssl -lcrypto
  else
    OPENSSL_CFLAGS =
    OPENSSL_LDFLAGS =
  endif
else
  OPENSSL_CFLAGS = -I$(OPENSSL_PREFIX)/include
  OPENSSL_LDFLAGS = -L$(OPENSSL_PREFIX)/lib -lssl -lcrypto
endif

# Auto-discover all .c files in runtime (excluding decimal subdirectory and desi_host)
RUNTIME_SRCS = $(filter-out $(RUNTIME_SRC)/desi_host.c,$(wildcard $(RUNTIME_SRC)/*.c))
RUNTIME_DB_SRCS = $(wildcard $(RUNTIME_DB)/*.c)
RUNTIME_OBJS = $(patsubst $(RUNTIME_SRC)/%.c,$(BUILD_DIR)/%.o,$(RUNTIME_SRCS))
RUNTIME_DB_OBJS = $(patsubst $(RUNTIME_DB)/%.c,$(BUILD_DIR)/%.o,$(RUNTIME_DB_SRCS))

LIB_DESI = $(BUILD_DIR)/libdesi.a

# Tools
DESIC = $(BIN_DIR)/desic
DESIFMT = $(BIN_DIR)/desifmt
DESIREPL = $(BIN_DIR)/desirepl
DESILSP = $(BIN_DIR)/desilsp

.PHONY: all clean runtime compiler tools directories decimal-lib
# Go binaries are PHONY because go build has its own cache;
# it only recompiles when source changes (fast no-op otherwise).
.PHONY: $(DESIC) $(DESIFMT) $(DESIREPL) $(DESILSP)

all: directories runtime compiler tools

# Fail early, and usefully, when a prerequisite is missing. Without this the
# build dies partway through with a raw toolchain error — a missing OpenSSL
# header surfaces as "compiler/runtime/http/tls_openssl.h:10:10: fatal error:
# openssl/ssl.h: No such file or directory" some forty files into the compile,
# which says nothing about what to install. bootstrap.sh already knows how to
# install all of this; this just points at it.
.PHONY: deps-check
deps-check:
	@missing=""; \
	command -v $(CC) >/dev/null 2>&1 || missing="$$missing  - a C compiler ($(CC))\n"; \
	command -v $(GO) >/dev/null 2>&1 || missing="$$missing  - Go\n"; \
	if [ ! -f /usr/include/openssl/ssl.h ] && \
	   [ ! -f /usr/local/include/openssl/ssl.h ] && \
	   ! pkg-config --exists openssl 2>/dev/null && \
	   [ -z "$$(brew --prefix openssl 2>/dev/null)" ]; then \
	    missing="$$missing  - OpenSSL development headers (libssl-dev / openssl-devel / brew openssl)\n"; \
	fi; \
	if [ -n "$$missing" ]; then \
	    printf "\nMissing build prerequisites:\n"; \
	    printf "$$missing"; \
	    printf "\nRun ./bootstrap.sh to install them, then try again.\n\n"; \
	    exit 1; \
	fi

directories: $(BUILD_DIR) $(BIN_DIR)

# Real directory targets so they can be used as order-only prerequisites
# below. Order-only (after the '|') means "must exist first" without the
# directory's mtime ever forcing a rebuild.
$(BUILD_DIR):
	@mkdir -p $(BUILD_DIR)

$(BIN_DIR):
	@mkdir -p $(BIN_DIR)

# Auto-build libmpdec if not present
decimal-lib: $(DECIMAL_LIB)

$(DECIMAL_LIB):
	@echo "==> Building libmpdec (first time setup)..."
	@mkdir -p $(DECIMAL_SRC)/lib $(DECIMAL_SRC)/include
	cd $(DECIMAL_SRC)/mpdecimal-4.0.1 && ./configure --quiet CFLAGS="$(CFLAGS)" && make -C libmpdec -s
	cp $(DECIMAL_SRC)/mpdecimal-4.0.1/libmpdec/libmpdec.a $(DECIMAL_SRC)/lib/
	cp $(DECIMAL_SRC)/mpdecimal-4.0.1/libmpdec/mpdecimal.h $(DECIMAL_SRC)/include/
	@echo "==> libmpdec built successfully"

# Runtime Library (includes decimal support)
runtime: deps-check decimal-lib $(LIB_DESI)

$(LIB_DESI): $(RUNTIME_OBJS) $(RUNTIME_DB_OBJS) $(BUILD_DIR)/desi_decimal.o
	@echo "==> Merging runtime + libmpdec into $@"
	@mkdir -p $(BUILD_DIR)/mpdec_objs
	@cd $(BUILD_DIR)/mpdec_objs && $(AR) x ../../$(DECIMAL_LIB)
	$(AR) rcs $@ $^ $(BUILD_DIR)/mpdec_objs/*.o

$(BUILD_DIR)/%.o: $(RUNTIME_SRC)/%.c | $(BUILD_DIR)
	@echo "==> Compiling $<..."
	$(CC) $(CFLAGS) $(OPENSSL_CFLAGS) -c $< -o $@

$(BUILD_DIR)/%.o: $(RUNTIME_DB)/%.c | $(BUILD_DIR)
	@echo "==> Compiling db/$<..."
	$(CC) $(CFLAGS) $(OPENSSL_CFLAGS) -c $< -o $@

$(BUILD_DIR)/desi_decimal.o: $(DECIMAL_SRC)/desi_decimal.c $(DECIMAL_LIB) | $(BUILD_DIR)
	@echo "==> Compiling decimal wrapper..."
	$(CC) $(CFLAGS) -I$(DECIMAL_SRC) -c $< -o $@

# Compiler and Tools
compiler: $(DESIC)

tools: $(DESIFMT) $(DESIREPL) $(DESILSP)

$(DESIC): | $(BIN_DIR)
	@echo "==> Building desic..."
	$(GO) build -ldflags="-s -w" -o $@ ./compiler/cmd/desic

$(DESIFMT): | $(BIN_DIR)
	@echo "==> Building desifmt..."
	$(GO) build -ldflags="-s -w" -o $@ ./compiler/cmd/desifmt

$(DESIREPL): | $(BIN_DIR)
	@echo "==> Building desirepl..."
	$(GO) build -ldflags="-s -w" -o $@ ./compiler/cmd/desirepl

$(DESILSP): | $(BIN_DIR)
	@echo "==> Building desilsp..."
	$(GO) build -ldflags="-s -w" -o $@ ./compiler/cmd/desilsp

# Hot Reload Host
DESIHOST = $(BIN_DIR)/desi-host

host: $(DESIHOST)

$(DESIHOST): $(RUNTIME_SRC)/desi_host.c
	@echo "==> Building desi-host..."
	$(CC) -o $@ $< -ldl


# Windows Build
windows:
	@mkdir -p $(BIN_DIR)
	@echo "==> Building Windows binaries..."
	GOOS=windows GOARCH=amd64 $(GO) build -ldflags="-s -w" -o $(BIN_DIR)/desic.exe ./compiler/cmd/desic
	GOOS=windows GOARCH=amd64 $(GO) build -ldflags="-s -w" -o $(BIN_DIR)/desifmt.exe ./compiler/cmd/desifmt
	GOOS=windows GOARCH=amd64 $(GO) build -ldflags="-s -w" -o $(BIN_DIR)/desirepl.exe ./compiler/cmd/desirepl
	GOOS=windows GOARCH=amd64 $(GO) build -ldflags="-s -w" -o $(BIN_DIR)/desilsp.exe ./compiler/cmd/desilsp

# Editor Plugins
.PHONY: vscode editors

vscode: $(DESILSP)
	@echo "==> Building VS Code extension..."
	cd editors/vscode && npm install && npm run package 2>/dev/null || npx vsce package --allow-missing-repository

editors: vscode
	@echo "==> All editor plugins built"

# Testing
.PHONY: test test-examples test-leaks asan

test:
	@echo "==> Running Go unit tests..."
	$(GO) test ./...
	@echo "==> Running example tests..."
	bash test_examples.sh

test-examples:
	bash test_examples.sh

# ASAN: Rebuild runtime with AddressSanitizer for leak/overflow detection
ASAN_CFLAGS = -fsanitize=address -fno-omit-frame-pointer -g
ASAN_RUNTIME_OBJS = $(patsubst $(RUNTIME_SRC)/%.c,$(BUILD_DIR)/asan_%.o,$(RUNTIME_SRCS))
ASAN_DB_OBJS = $(patsubst $(RUNTIME_DB)/%.c,$(BUILD_DIR)/asan_%.o,$(RUNTIME_DB_SRCS))

asan: directories decimal-lib
	@echo "==> Building ASAN runtime..."
	@for src in $(RUNTIME_SRCS); do \
		obj=$(BUILD_DIR)/asan_$$(basename $$src .c).o; \
		echo "  CC [asan] $$src"; \
		$(CC) $(ASAN_CFLAGS) $(OPENSSL_CFLAGS) -c $$src -o $$obj; \
	done
	@for src in $(RUNTIME_DB_SRCS); do \
		obj=$(BUILD_DIR)/asan_$$(basename $$src .c).o; \
		echo "  CC [asan] $$src"; \
		$(CC) $(ASAN_CFLAGS) $(OPENSSL_CFLAGS) -c $$src -o $$obj; \
	done
	$(CC) $(ASAN_CFLAGS) -I$(DECIMAL_SRC) -c $(DECIMAL_SRC)/desi_decimal.c -o $(BUILD_DIR)/asan_desi_decimal.o
	@echo "==> Merging ASAN runtime..."
	@mkdir -p $(BUILD_DIR)/mpdec_objs
	@cd $(BUILD_DIR)/mpdec_objs && $(AR) x ../../$(DECIMAL_LIB)
	$(AR) rcs $(BUILD_DIR)/libdesi_asan.a $(BUILD_DIR)/asan_*.o $(BUILD_DIR)/mpdec_objs/*.o
	@echo "✓ ASAN runtime built: $(BUILD_DIR)/libdesi_asan.a"
	@echo "  Use: clang -fsanitize=address program.o -L$(BUILD_DIR) -ldesi_asan -o test_prog"

test-leaks:
	@echo "==> Running leak detection tests..."
	bash test_leaks.sh

clean:
	rm -rf $(BUILD_DIR) $(BIN_DIR) gen/
