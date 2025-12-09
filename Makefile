# Makefile for Desi Language
# Supports cross-platform builds via GOOS/GOARCH and CC/AR overrides

# Variables
CC ?= clang
AR ?= ar
GO ?= go
BUILD_DIR = build
BIN_DIR = bin
RUNTIME_SRC = compiler/runtime
RUNTIME_OBJS = $(BUILD_DIR)/set.o $(BUILD_DIR)/dict.o $(BUILD_DIR)/print.o $(BUILD_DIR)/string.o $(BUILD_DIR)/list.o $(BUILD_DIR)/file.o $(BUILD_DIR)/arena.o
LIB_DESI = $(BUILD_DIR)/libdesi.a

# Tools
DESIC = $(BIN_DIR)/desic
DESIFMT = $(BIN_DIR)/desifmt
DESIREPL = $(BIN_DIR)/desirepl

.PHONY: all clean runtime compiler tools directories

all: directories runtime compiler tools

directories:
	@mkdir -p $(BUILD_DIR) $(BIN_DIR)

# Runtime Library
runtime: $(LIB_DESI)

$(LIB_DESI): $(RUNTIME_OBJS)
	@echo "==> Archiving runtime library to $@"
	$(AR) rcs $@ $^

$(BUILD_DIR)/%.o: $(RUNTIME_SRC)/%.c
	@echo "==> Compiling $<..."
	$(CC) -c $< -o $@

# Compiler and Tools
compiler: $(DESIC)

tools: $(DESIFMT) $(DESIREPL)

$(DESIC):
	@echo "==> Building desic..."
	$(GO) build -o $@ ./compiler/cmd/desic

$(DESIFMT):
	@echo "==> Building desifmt..."
	$(GO) build -o $@ ./compiler/cmd/desifmt

$(DESIREPL):
	@echo "==> Building desirepl..."
	$(GO) build -o $@ ./compiler/cmd/desirepl

# Windows Build
windows:
	@mkdir -p $(BIN_DIR)
	@echo "==> Building Windows binaries..."
	GOOS=windows GOARCH=amd64 $(GO) build -o $(BIN_DIR)/desic.exe ./compiler/cmd/desic
	GOOS=windows GOARCH=amd64 $(GO) build -o $(BIN_DIR)/desifmt.exe ./compiler/cmd/desifmt
	GOOS=windows GOARCH=amd64 $(GO) build -o $(BIN_DIR)/desirepl.exe ./compiler/cmd/desirepl

clean:
	rm -rf $(BUILD_DIR) $(BIN_DIR) gen/
