//go:build linux && amd64

package abi

func init() {
	Register(&Info{
		// Linux x86_64 target triple
		TargetTriple: "x86_64-unknown-linux-gnu",

		// Linux x86_64 datalayout
		// e = little endian
		// m:e = ELF mangling
		// p270:32:32 = address space 270 pointers are 32-bit with 32-bit alignment
		// p271:32:32 = address space 271 pointers are 32-bit with 32-bit alignment
		// p272:64:64 = address space 272 pointers are 64-bit with 64-bit alignment
		// i64:64 = 64-bit integers are 64-bit aligned
		// f80:128 = 80-bit floats are 128-bit aligned
		// n8:16:32:64 = native integer widths are 8, 16, 32, and 64 bits
		// S128 = stack alignment is 128 bits (16 bytes)
		TargetLayout: "e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-f80:128-n8:16:32:64-S128",

		// x86_64 uses register-based variadic argument passing (System V AMD64 ABI)
		// First 6 integer/pointer args in registers (rdi, rsi, rdx, rcx, r8, r9)
		// First 8 floating-point args in XMM registers
		// Remaining args on stack
		VariadicConv: RegisterBased,
	})
}
