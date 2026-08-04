package llvm

// The parts of DesiList the code generator reads directly.
//
// `xs[i]` does not lower to a call any more: the backend emits the bounds check
// and the load inline and only calls the runtime when the check fails. That
// means these offsets are an ABI between compiler/runtime/list.h and the code
// generator, not a private detail of either. Reordering a field or changing its
// width there would silently change what every generated program loads.
//
// Both sides are pinned. list.h carries _Static_asserts that fail the build,
// and list_layout_test.go compiles a C program against the real header and
// compares what it reports with the constants below, so a change on either side
// is caught rather than mis-compiled.
const (
	// ListDataOffset is where the element array pointer lives. It is first, so
	// the backend loads it with a bare `load ptr, ptr %list`.
	ListDataOffset = 0

	// ListLengthOffset is the live element count, loaded as i64 for the bounds
	// check and stored back by an append.
	ListLengthOffset = 8

	// ListCapacityOffset is the allocated slot count, compared against length
	// to decide whether an append can store in place or has to grow.
	ListCapacityOffset = 16

	// ListTypeTagOffset records what the list holds. An append promotes it from
	// 0 (int, the default a fresh list starts at) the first time something else
	// arrives, which is why an inline append has to maintain it rather than
	// leave it to the runtime.
	ListTypeTagOffset = 24
)
