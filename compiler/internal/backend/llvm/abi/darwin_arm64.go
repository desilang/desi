//go:build darwin && arm64

package abi

func init() {
	Register(&Info{
		// macOS ARM64 target triple
		// See: https://llvm.org/docs/LangRef.html#target-triple
		TargetTriple: "arm64-apple-macosx14.0.0",

		// macOS ARM64 datalayout
		// e = little endian
		// m:o = Mach-O mangling
		// i64:64 = 64-bit integers are 64-bit aligned
		// i128:128 = 128-bit integers are 128-bit aligned
		// n32:64 = native integer widths are 32 and 64 bits
		// S128 = stack alignment is 128 bits (16 bytes)
		TargetLayout: "e-m:o-i64:64-i128:128-n32:64-S128",

		// macOS ARM64 uses stack-based variadic argument passing
		// This is different from standard ARM64 AAPCS which uses registers
		// See: https://developer.apple.com/documentation/xcode/writing-arm64-code-for-apple-platforms
		VariadicConv: StackBased,
	})
}
