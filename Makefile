MAKEFLAGS += --no-builtin-rules --warn-undefined-variables

GO        := go
DESIC     := $(GO) run ./compiler/cmd/desic
DESIFMT   := $(GO) run ./compiler/cmd/desifmt
DESIREPL  := $(GO) run ./compiler/cmd/desirepl

# All tracked .desi files (may be empty).
DESI_FILES := $(shell git ls-files '*.desi')

.PHONY: build test tokens demo-layout fmt repl

build:
	$(GO) build ./...

test:
	$(GO) test ./...

tokens:
	$(DESIC) -tokens examples/14_m7_main.desi

demo-layout:
	$(DESIC) -demo-layout examples/14_m7_main.desi

fmt:
	@files="$(DESI_FILES)"; \
	if [ -z "$$files" ]; then \
		echo "fmt: no .desi files found"; \
	else \
		$(DESIFMT) -w $$files; \
	fi

repl:
	$(DESIREPL)
