package intrin

import "fmt"

// LifetimeStart returns an LLVM IR call to llvm.lifetime.start for a pointer local.
//
// Example:
//
//	LifetimeStart(32, "buf") -> "  call void @llvm.lifetime.start.p0(i64 32, ptr %buf)\n"
func LifetimeStart(size int, ptrName string) string {
	return fmt.Sprintf("  call void @llvm.lifetime.start.p0(i64 %d, ptr %%%s)\n", size, ptrName)
}

// LifetimeEnd returns an LLVM IR call to llvm.lifetime.end for a pointer local.
//
// Example:
//
//	LifetimeEnd(32, "buf") -> "  call void @llvm.lifetime.end.p0(i64 32, ptr %buf)\n"
func LifetimeEnd(size int, ptrName string) string {
	return fmt.Sprintf("  call void @llvm.lifetime.end.p0(i64 %d, ptr %%%s)\n", size, ptrName)
}
