// file_lower.go - Lower file I/O operations to HIR
package lower

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/hir"
	"github.com/desilang/desi/compiler/internal/types"
)

// lowerFileMethod handles File.read(), File.write(), File.close()
func (ls *lowerState) lowerFileMethod(fe *ast.FieldExpr, args []ast.Expr) hir.Value {
	receiver := ls.lowerExpr(fe.X)
	method := fe.Name.Name

	switch method {
	case "read":
		// file_read_all(File, &out, &err) -> int (0=ok, 1=err)
		// For now, simplified: just return the string, panic on error
		outPtr := ls.b.FreshTemp("read_out")
		errPtr := ls.b.FreshTemp("read_err")

		// Allocate stack space for output pointers
		ls.b.Emit(&hir.Alloca{Dst: outPtr, Type: "ptr"})
		ls.b.Emit(&hir.Alloca{Dst: errPtr, Type: "ptr"})

		resultTmp := ls.b.FreshTemp("read_result")
		ls.b.Emit(&hir.Call{
			Fn:   "file_read_all",
			Args: []hir.Value{receiver, outPtr, errPtr},
			Dst:  resultTmp,
			Type: "i32", // returns int error code
		})

		// Load the string result
		strResult := ls.b.FreshTemp("read_str")
		ls.b.Emit(&hir.Load{Dst: strResult, Src: outPtr, Type: "ptr"})
		return strResult

	case "write":
		if len(args) < 1 {
			return hir.ConstInt{Text: "0"}
		}
		data := ls.lowerExpr(args[0])
		errPtr := ls.b.FreshTemp("write_err")
		ls.b.Emit(&hir.Alloca{Dst: errPtr, Type: "ptr"})

		resultTmp := ls.b.FreshTemp("write_result")
		ls.b.Emit(&hir.Call{
			Fn:   "file_write",
			Args: []hir.Value{receiver, data, errPtr},
			Dst:  resultTmp,
			Type: "i32", // returns int error code
		})
		return resultTmp

	case "close":
		ls.b.Emit(&hir.Call{
			Fn:   "file_close",
			Args: []hir.Value{receiver},
			Type: "void",
		})
		return hir.ConstInt{Text: "0"}

	case "is_open":
		resultTmp := ls.b.FreshTemp("is_open")
		ls.b.Emit(&hir.Call{
			Fn:   "file_is_open",
			Args: []hir.Value{receiver},
			Dst:  resultTmp,
			Type: "i32", // returns int (boolean)
		})
		return resultTmp
	}

	return hir.ConstInt{Text: "0"}
}

// lowerFileOpen handles the open(path, mode) builtin
func (ls *lowerState) lowerFileOpen(args []ast.Expr) hir.Value {
	if len(args) < 2 {
		return hir.ConstInt{Text: "0"}
	}

	path := ls.lowerExpr(args[0])
	mode := ls.lowerExpr(args[1])

	// Allocate error pointer
	errPtr := ls.b.FreshTemp("open_err")
	ls.b.Emit(&hir.Alloca{Dst: errPtr, Type: "ptr"})

	// Store null in error pointer initially
	ls.b.Emit(&hir.Store{Dst: errPtr, Val: hir.ConstInt{Text: "0"}})

	// Call file_open - IMPORTANT: returns ptr (DesiFile*), not i32!
	result := ls.b.FreshTemp("file_handle")
	ls.b.Emit(&hir.Call{
		Fn:   "file_open",
		Args: []hir.Value{path, mode, errPtr},
		Dst:  result,
		Type: "ptr", // Critical: file handle is a pointer
	})

	return result
}

// isFileType checks if a type is the File type
func isFileType(t types.T) bool {
	return types.Equal(t, types.File)
}

// isMutexGuardType checks if a type is MutexGuard[T]
func isMutexGuardType(t types.T) bool {
	_, ok := t.(*types.MutexGuard)
	return ok
}

// isReadGuardType checks if a type is ReadGuard[T]
func isReadGuardType(t types.T) bool {
	_, ok := t.(*types.ReadGuard)
	return ok
}

// isWriteGuardType checks if a type is WriteGuard[T]
func isWriteGuardType(t types.T) bool {
	_, ok := t.(*types.WriteGuard)
	return ok
}

// isSenderType checks if a type is ChannelSender[T]
func isSenderType(t types.T) bool {
	_, ok := t.(*types.ChannelSender)
	return ok
}

// isReceiverType checks if a type is ChannelReceiver[T]
func isReceiverType(t types.T) bool {
	_, ok := t.(*types.ChannelReceiver)
	return ok
}
