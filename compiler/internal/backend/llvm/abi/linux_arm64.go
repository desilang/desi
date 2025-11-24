//go:build linux && arm64

package abi

func init() {
	Register(&Info{
		// Linux ARM64 target triple (AARCH64)
		TargetTriple: "aarch64-unknown-linux-gnu",

		// Linux ARM64 datalayout
		// e = little endian
		// m:e = ELF mangling
		// i8:8:32 = 8-bit integers have 8-bit size, 32-bit alignment
		// i16:16:32 = 16-bit integers have 16-bit size, 32-bit alignment
		// i64:64 = 64-bit integers are 64-bit aligned
		// i128:128 = 128-bit integers are 128-bit aligned
		// n32:64 = native integer widths are 32 and 64 bits
		// S128 = stack alignment is 128 bits (16 bytes)
		TargetLayout: "e-m:e-i8:8:32-i16:16:32-i64:64-i128:128-n32:64-S128",

		// Linux ARM64 uses stack-based variadic argument passing (AAPCS64)
		// Similar to macOS ARM64 but with ELF conventions
		VariadicConv: StackBased,
	})
}
