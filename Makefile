# Makefile for Desi Language
# Supports cross-platform builds via GOOS/GOARCH and CC/AR overrides

# Variables
CC ?= clang
AR ?= ar
GO ?= go
BUILD_DIR = build
BIN_DIR = bin
RUNTIME_SRC = compiler/runtime
DECIMAL_SRC = compiler/runtime/decimal
DECIMAL_LIB = $(DECIMAL_SRC)/lib/libmpdec.a

# Auto-discover all .c files in runtime (excluding decimal subdirectory)
RUNTIME_SRCS = $(wildcard $(RUNTIME_SRC)/*.c)
RUNTIME_OBJS = $(patsubst $(RUNTIME_SRC)/%.c,$(BUILD_DIR)/%.o,$(RUNTIME_SRCS))

LIB_DESI = $(BUILD_DIR)/libdesi.a

# Tools
DESIC = $(BIN_DIR)/desic
DESIFMT = $(BIN_DIR)/desifmt
DESIREPL = $(BIN_DIR)/desirepl

.PHONY: all clean runtime compiler tools directories decimal-lib

all: directories runtime compiler tools

directories:
	@mkdir -p $(BUILD_DIR) $(BIN_DIR)

# Auto-build libmpdec if not present
decimal-lib: $(DECIMAL_LIB)

$(DECIMAL_LIB):
	@echo "==> Building libmpdec (first time setup)..."
	@mkdir -p $(DECIMAL_SRC)/lib $(DECIMAL_SRC)/include
	cd $(DECIMAL_SRC)/mpdecimal-4.0.1 && ./configure --quiet && make -C libmpdec -s
	cp $(DECIMAL_SRC)/mpdecimal-4.0.1/libmpdec/libmpdec.a $(DECIMAL_SRC)/lib/
	cp $(DECIMAL_SRC)/mpdecimal-4.0.1/libmpdec/mpdecimal.h $(DECIMAL_SRC)/include/
	@echo "==> libmpdec built successfully"

# Runtime Library (includes decimal support)
runtime: decimal-lib $(LIB_DESI)

$(LIB_DESI): $(RUNTIME_OBJS) $(BUILD_DIR)/desi_decimal.o
	@echo "==> Merging runtime + libmpdec into $@"
	cp $(DECIMAL_LIB) $@
	$(AR) rcs $@ $^

$(BUILD_DIR)/%.o: $(RUNTIME_SRC)/%.c
	@echo "==> Compiling $<..."
	$(CC) -c $< -o $@

$(BUILD_DIR)/desi_decimal.o: $(DECIMAL_SRC)/desi_decimal.c $(DECIMAL_LIB)
	@echo "==> Compiling decimal wrapper..."
	$(CC) -I$(DECIMAL_SRC) -c $< -o $@

# Compiler and Tools
compiler: $(DESIC)

tools: $(DESIFMT) $(DESIREPL)

$(DESIC):
	@echo "==> Building desic..."
	$(GO) build -ldflags="-s -w" -o $@ ./compiler/cmd/desic

$(DESIFMT):
	@echo "==> Building desifmt..."
	$(GO) build -ldflags="-s -w" -o $@ ./compiler/cmd/desifmt

$(DESIREPL):
	@echo "==> Building desirepl..."
	$(GO) build -ldflags="-s -w" -o $@ ./compiler/cmd/desirepl

# Windows Build
windows:
	@mkdir -p $(BIN_DIR)
	@echo "==> Building Windows binaries..."
	GOOS=windows GOARCH=amd64 $(GO) build -ldflags="-s -w" -o $(BIN_DIR)/desic.exe ./compiler/cmd/desic
	GOOS=windows GOARCH=amd64 $(GO) build -ldflags="-s -w" -o $(BIN_DIR)/desifmt.exe ./compiler/cmd/desifmt
	GOOS=windows GOARCH=amd64 $(GO) build -ldflags="-s -w" -o $(BIN_DIR)/desirepl.exe ./compiler/cmd/desirepl

clean:
	rm -rf $(BUILD_DIR) $(BIN_DIR) gen/

