package llvm

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/desilang/desi/compiler/internal/backend/llvm/abi"
	"github.com/desilang/desi/compiler/internal/hir"
	"github.com/desilang/desi/compiler/internal/types"
)

func (m *Module) emitCall(c *hir.Call) {
	// Check if this function belongs to a lazy module - emit init thunk if needed
	m.ensureLazyModuleInit(c.Fn)

	// bool_to_cstring(int): i1 arguments must be zero-extended to i32.
	// Passed through a variadic declare, an i1 leaves the upper register
	// bits undefined on Windows x64 and the C `int` parameter reads
	// garbage — false formats as "true". Same ABI fix as bool_to_str.
	if c.Fn == "bool_to_cstring" && len(c.Args) == 1 {
		m.ensureDecl("declare ptr @bool_to_cstring(i32)")
		ty, val := m.operand(c.Args[0])
		dst := c.Dst.Name
		if dst == "" {
			dst = fmt.Sprintf("%%t%d", m.tempID)
			m.tempID++
		}
		argOp := val
		switch ty {
		case "i1":
			ext := fmt.Sprintf("%%bext_%d", m.tempID)
			m.tempID++
			wprintf(&m.funcs, "  %s = zext i1 %s to i32\n", ext, val)
			argOp = ext
		case "i64":
			ext := fmt.Sprintf("%%bext_%d", m.tempID)
			m.tempID++
			wprintf(&m.funcs, "  %s = trunc i64 %s to i32\n", ext, val)
			argOp = ext
		}
		wprintf(&m.funcs, "  %s = call ptr @bool_to_cstring(i32 %s)\n", dst, argOp)
		m.tempTypes[strings.TrimPrefix(dst, "%")] = "ptr"
		return
	}

	// Stream accessor functions: __get_stdout(), __get_stderr()
	if c.Fn == "__get_stdout" && len(c.Args) == 0 {
		m.ensureDecl("declare ptr @__get_stdout()")
		dst := c.Dst.Name
		if dst == "" {
			dst = fmt.Sprintf("%%t%d", m.tempID)
			m.tempID++
		}
		wprintf(&m.funcs, "  %s = call ptr @__get_stdout()\n", dst)
		return
	}
	if c.Fn == "__get_stderr" && len(c.Args) == 0 {
		m.ensureDecl("declare ptr @__get_stderr()")
		dst := c.Dst.Name
		if dst == "" {
			dst = fmt.Sprintf("%%t%d", m.tempID)
			m.tempID++
		}
		wprintf(&m.funcs, "  %s = call ptr @__get_stderr()\n", dst)
		return
	}

	// Convert DesiFile* to DesiStream* for print compatibility
	if c.Fn == "desifile_get_stream" && len(c.Args) == 1 {
		m.ensureDecl("declare ptr @desifile_get_stream(ptr)")
		_, fileVal := m.operand(c.Args[0])
		dst := c.Dst.Name
		if dst == "" {
			dst = fmt.Sprintf("%%t%d", m.tempID)
			m.tempID++
		}
		wprintf(&m.funcs, "  %s = call ptr @desifile_get_stream(ptr %s)\n", dst, fileVal)
		return
	}

	// Stream print: stream_print_str(stream, str)
	if c.Fn == "stream_print_str" && len(c.Args) == 2 {
		m.ensureDecl("declare void @stream_print_str(ptr, ptr)")
		_, streamVal := m.operand(c.Args[0])
		if s, ok := c.Args[1].(hir.ConstStr); ok {
			strG, strN := m.ensureCStringGlobal(s.Text, false)
			wprintf(&m.funcs, "  %%t%d = getelementptr inbounds [%d x i8], [%d x i8]* %s, i64 0, i64 0\n",
				m.tempID, strN, strN, strG)
			wprintf(&m.funcs, "  call void @stream_print_str(ptr %s, ptr %%t%d)\n", streamVal, m.tempID)
			m.tempID++
		} else {
			_, strVal := m.operand(c.Args[1])
			wprintf(&m.funcs, "  call void @stream_print_str(ptr %s, ptr %s)\n", streamVal, strVal)
		}
		return
	}

	// Stream flush: stream_flush(stream)
	if c.Fn == "stream_flush" && len(c.Args) == 1 {
		m.ensureDecl("declare void @stream_flush(ptr)")
		_, streamVal := m.operand(c.Args[0])
		wprintf(&m.funcs, "  call void @stream_flush(ptr %s)\n", streamVal)
		return
	}

	// Print style start: set ANSI color for stdout
	if c.Fn == "print_style_start_stdout" && len(c.Args) == 1 {
		m.ensureDecl("declare void @print_style_start_stdout(ptr)")
		if s, ok := c.Args[0].(hir.ConstStr); ok {
			strG, strN := m.ensureCStringGlobal(s.Text, false)
			wprintf(&m.funcs, "  %%t%d = getelementptr inbounds [%d x i8], [%d x i8]* %s, i64 0, i64 0\n",
				m.tempID, strN, strN, strG)
			wprintf(&m.funcs, "  call void @print_style_start_stdout(ptr %%t%d)\n", m.tempID)
			m.tempID++
		} else {
			_, styleVal := m.operand(c.Args[0])
			wprintf(&m.funcs, "  call void @print_style_start_stdout(ptr %s)\n", styleVal)
		}
		return
	}

	// Print style end: reset ANSI colors for stdout
	if c.Fn == "print_style_end_stdout" && len(c.Args) == 0 {
		m.ensureDecl("declare void @print_style_end_stdout()")
		wprintf(&m.funcs, "  call void @print_style_end_stdout()\n")
		return
	}

	// Note: log_info/warn/error/debug/fatal/set_level are handled
	// through the normal module import system (log.desi → __log_* C runtime).

	// Sync module runtime functions: mutex, channel, taskgroup
	// These have explicit declarations, mark as defined to skip variadic fallback
	switch c.Fn {
	case "mutex_new":
		m.ensureDecl("declare ptr @mutex_new(ptr)")
		m.definedFunctions["mutex_new"] = true
	case "mutex_lock":
		m.ensureDecl("declare ptr @mutex_lock(ptr)")
		m.definedFunctions["mutex_lock"] = true
	case "mutex_try_lock":
		m.ensureDecl("declare ptr @mutex_try_lock(ptr)")
		m.definedFunctions["mutex_try_lock"] = true
	case "mutex_unlock":
		m.ensureDecl("declare void @mutex_unlock(ptr)")
		m.definedFunctions["mutex_unlock"] = true
	case "mutex_guard_get":
		m.ensureDecl("declare ptr @mutex_guard_get(ptr)")
		m.definedFunctions["mutex_guard_get"] = true
	case "channel_new":
		m.ensureDecl("declare ptr @channel_new(i64)")
		m.definedFunctions["channel_new"] = true
	case "channel_sender":
		m.ensureDecl("declare ptr @channel_sender(ptr)")
		m.definedFunctions["channel_sender"] = true
	case "channel_receiver":
		m.ensureDecl("declare ptr @channel_receiver(ptr)")
		m.definedFunctions["channel_receiver"] = true
	case "channel_send":
		m.ensureDecl("declare i1 @channel_send(ptr, ptr)")
		m.definedFunctions["channel_send"] = true
	case "channel_close":
		m.ensureDecl("declare void @channel_close(ptr)")
		m.definedFunctions["channel_close"] = true
	case "taskgroup_new":
		m.ensureDecl("declare ptr @taskgroup_new()")
		m.definedFunctions["taskgroup_new"] = true
	case "taskgroup_wait":
		m.ensureDecl("declare void @taskgroup_wait(ptr)")
		m.definedFunctions["taskgroup_wait"] = true
	case "taskgroup_is_cancelled":
		m.ensureDecl("declare i1 @taskgroup_is_cancelled(ptr)")
		m.definedFunctions["taskgroup_is_cancelled"] = true
	// Exception handling runtime
	case "setjmp":
		abiInfo := abi.Current()
		if abiInfo.IsWindows() {
			// Windows x64 MSVC: setjmp must be lowered to _setjmp(buf, frameaddr).
			// Clang does this expansion automatically; since we write IR directly we
			// must replicate it. Three things are required:
			//
			//   1. Alloca the i32 result slot BEFORE frameaddress + _setjmp calls.
			//      LLVM may treat allocas AFTER a returns_twice call as dynamic
			//      (non-hoisted) allocas, which breaks the store/load pattern.
			//   2. @llvm.frameaddress.p0(i32 0) → frame pointer for RtlUnwindEx
			//   3. call i32 @_setjmp(ptr %buf, ptr %fp) returns_twice [on call site]
			//   4. Store/reload result via the alloca slot so the value is re-read
			//      from memory after longjmp (registers are stale, memory survives).
			//
			// Without (1): second+ setjmps in non-entry blocks crash at runtime
			// Without (2): longjmp calls RtlUnwindEx(Frame=0) → STATUS_ACCESS_VIOLATION
			// Without (4): after longjmp, setjmp result stays 0 → try body re-runs
			m.ensureDecl("declare i32 @_setjmp(ptr, ptr) returns_twice")
			m.ensureDecl("declare ptr @llvm.frameaddress.p0(i32)")
			m.definedFunctions["setjmp"] = true

			dst := c.Dst.Name
			if dst == "" {
				dst = fmt.Sprintf("%%t%d", m.tempID)
				m.tempID++
			}

			// 1. Alloca slot FIRST (before any calls) so LLVM hoists it to prologue
			slotTemp := fmt.Sprintf("%%t%d", m.tempID)
			m.tempID++
			wprintf(&m.funcs, "  %s = alloca i32, align 4\n", slotTemp)

			// 2. Get the frame pointer
			fpTemp := fmt.Sprintf("%%t%d", m.tempID)
			m.tempID++
			wprintf(&m.funcs, "  %s = call ptr @llvm.frameaddress.p0(i32 0)\n", fpTemp)

			// 3. Get the buf pointer
			_, bufVal := m.operand(c.Args[0])

			// 4. Call _setjmp — result into a raw temp
			rawTemp := fmt.Sprintf("%%t%d", m.tempID)
			m.tempID++
			wprintf(&m.funcs, "  %s = call i32 @_setjmp(ptr %s, ptr %s) returns_twice\n",
				rawTemp, bufVal, fpTemp)

			// 5. Store + reload so the value survives longjmp stack restoration
			wprintf(&m.funcs, "  store i32 %s, ptr %s\n", rawTemp, slotTemp)
			wprintf(&m.funcs, "  %s = load i32, ptr %s\n", dst, slotTemp)

			if m.tempTypes == nil {
				m.tempTypes = make(map[string]string)
			}
			m.tempTypes[strings.TrimPrefix(dst, "%")] = "i32"
			return
		}
		// POSIX: standard 1-arg setjmp with returns_twice
		m.ensureDecl("declare i32 @setjmp(ptr) returns_twice")
		m.definedFunctions["setjmp"] = true
	case "__desi_try_push":
		m.ensureDecl("declare void @__desi_try_push(ptr)")
		m.definedFunctions["__desi_try_push"] = true
	case "__desi_try_pop":
		m.ensureDecl("declare void @__desi_try_pop()")
		m.definedFunctions["__desi_try_pop"] = true
	case "__desi_raise":
		m.ensureDecl("declare void @__desi_raise(i32, ptr, ptr)")
		m.definedFunctions["__desi_raise"] = true
	case "__desi_reraise":
		m.ensureDecl("declare void @__desi_reraise()")
		m.definedFunctions["__desi_reraise"] = true
	case "__desi_exception_tag":
		m.ensureDecl("declare i32 @__desi_exception_tag()")
		m.definedFunctions["__desi_exception_tag"] = true
	case "__desi_exception_message":
		m.ensureDecl("declare ptr @__desi_exception_message()")
		m.definedFunctions["__desi_exception_message"] = true
	case "__desi_exception_matches":
		m.ensureDecl("declare i32 @__desi_exception_matches(i32)")
		m.definedFunctions["__desi_exception_matches"] = true
	}

	// Marshal dumps: __marshal_dumps(val, type) -> ptr (DesiBytes)
	if c.Fn == "__marshal_dumps" && len(c.Args) == 2 {
		m.ensureDecl("declare ptr @__marshal_dumps(ptr, ptr)")
		dst := c.Dst.Name
		_, valOp := m.operand(c.Args[0])
		_, typeOp := m.operand(c.Args[1])
		wprintf(&m.funcs, "  %s = call ptr @__marshal_dumps(ptr %s, ptr %s)\n", dst, valOp, typeOp)
		if m.tempTypes == nil {
			m.tempTypes = make(map[string]string)
		}
		m.tempTypes[strings.TrimPrefix(dst, "%")] = "ptr"
		return
	}

	// Marshal loads: __marshal_loads(data, type) -> ptr
	if c.Fn == "__marshal_loads" && len(c.Args) == 2 {
		m.ensureDecl("declare ptr @__marshal_loads(ptr, ptr)")
		dst := c.Dst.Name
		_, dataOp := m.operand(c.Args[0])
		_, typeOp := m.operand(c.Args[1])
		wprintf(&m.funcs, "  %s = call ptr @__marshal_loads(ptr %s, ptr %s)\n", dst, dataOp, typeOp)
		if m.tempTypes == nil {
			m.tempTypes = make(map[string]string)
		}
		m.tempTypes[strings.TrimPrefix(dst, "%")] = "ptr"
		return
	}

	// JSON parse: __json_parse(text) -> ptr
	if c.Fn == "__json_parse" && len(c.Args) == 1 {
		m.ensureDecl("declare ptr @__json_parse(ptr)")
		dst := c.Dst.Name
		if s, ok := c.Args[0].(hir.ConstStr); ok {
			strG, strN := m.ensureCStringGlobal(s.Text, false)
			wprintf(&m.funcs, "  %%t%d = getelementptr inbounds [%d x i8], [%d x i8]* %s, i64 0, i64 0\n",
				m.tempID, strN, strN, strG)
			wprintf(&m.funcs, "  %s = call ptr @__json_parse(ptr %%t%d)\n", dst, m.tempID)
			m.tempID++
		} else {
			_, textVal := m.operand(c.Args[0])
			wprintf(&m.funcs, "  %s = call ptr @__json_parse(ptr %s)\n", dst, textVal)
		}
		m.tempTypes[strings.TrimPrefix(dst, "%")] = "ptr"
		return
	}

	// JSON stringify: __json_stringify(node) -> ptr (string)
	if c.Fn == "__json_stringify" && len(c.Args) == 1 {
		m.ensureDecl("declare ptr @__json_stringify(ptr)")
		dst := c.Dst.Name
		_, nodeVal := m.operand(c.Args[0])
		wprintf(&m.funcs, "  %s = call ptr @__json_stringify(ptr %s)\n", dst, nodeVal)
		// Store return type for later type inference
		if m.tempTypes == nil {
			m.tempTypes = make(map[string]string)
		}
		m.tempTypes[strings.TrimPrefix(dst, "%")] = "ptr"
		return
	}

	// JSON type check: __json_is_* -> i1 (bool)
	if strings.HasPrefix(c.Fn, "__json_is_") && len(c.Args) == 1 {
		m.ensureDecl("declare i1 @" + c.Fn + "(ptr)")
		dst := c.Dst.Name
		_, nodeVal := m.operand(c.Args[0])
		wprintf(&m.funcs, "  %s = call i1 @%s(ptr %s)\n", dst, c.Fn, nodeVal)
		// Store result type for branch instructions
		if m.tempTypes == nil {
			m.tempTypes = make(map[string]string)
		}
		m.tempTypes[strings.TrimPrefix(dst, "%")] = "i1"
		return
	}

	// JSON get type: __json_type(node) -> i32
	if c.Fn == "__json_type" && len(c.Args) == 1 {
		m.ensureDecl("declare i32 @__json_type(ptr)")
		dst := c.Dst.Name
		_, nodeVal := m.operand(c.Args[0])
		wprintf(&m.funcs, "  %s = call i32 @__json_type(ptr %s)\n", dst, nodeVal)
		m.tempTypes[strings.TrimPrefix(dst, "%")] = "i32"
		return
	}

	// JSON get bool: __json_get_bool(node) -> i1
	if c.Fn == "__json_get_bool" && len(c.Args) == 1 {
		m.ensureDecl("declare i1 @__json_get_bool(ptr)")
		dst := c.Dst.Name
		_, nodeVal := m.operand(c.Args[0])
		wprintf(&m.funcs, "  %s = call i1 @__json_get_bool(ptr %s)\n", dst, nodeVal)
		// Store result type for branch instructions
		if m.tempTypes == nil {
			m.tempTypes = make(map[string]string)
		}
		m.tempTypes[strings.TrimPrefix(dst, "%")] = "i1"
		return
	}

	// JSON get number: __json_get_number(node) -> double
	if c.Fn == "__json_get_number" && len(c.Args) == 1 {
		m.ensureDecl("declare double @__json_get_number(ptr)")
		dst := c.Dst.Name
		_, nodeVal := m.operand(c.Args[0])
		wprintf(&m.funcs, "  %s = call double @__json_get_number(ptr %s)\n", dst, nodeVal)
		// Store result type for f-string formatting
		if m.tempTypes == nil {
			m.tempTypes = make(map[string]string)
		}
		m.tempTypes[strings.TrimPrefix(dst, "%")] = "double"
		return
	}

	// JSON get float: __json_get_float(node) -> double (alias for get_number)
	if c.Fn == "__json_get_float" && len(c.Args) == 1 {
		m.ensureDecl("declare double @__json_get_float(ptr)")
		dst := c.Dst.Name
		_, nodeVal := m.operand(c.Args[0])
		wprintf(&m.funcs, "  %s = call double @__json_get_float(ptr %s)\n", dst, nodeVal)
		if m.tempTypes == nil {
			m.tempTypes = make(map[string]string)
		}
		m.tempTypes[strings.TrimPrefix(dst, "%")] = "double"
		return
	}

	// JSON get int: __json_get_int(node) -> i32 (Desi int = i32)
	if c.Fn == "__json_get_int" && len(c.Args) == 1 {
		m.ensureDecl("declare i32 @__json_get_int(ptr)")
		dst := c.Dst.Name
		_, nodeVal := m.operand(c.Args[0])
		wprintf(&m.funcs, "  %s = call i32 @__json_get_int(ptr %s)\n", dst, nodeVal)
		if m.tempTypes == nil {
			m.tempTypes = make(map[string]string)
		}
		m.tempTypes[strings.TrimPrefix(dst, "%")] = "i32"
		return
	}

	// JSON get string: __json_get_string(node) -> ptr
	if c.Fn == "__json_get_string" && len(c.Args) == 1 {
		m.ensureDecl("declare ptr @__json_get_string(ptr)")
		dst := c.Dst.Name
		_, nodeVal := m.operand(c.Args[0])
		wprintf(&m.funcs, "  %s = call ptr @__json_get_string(ptr %s)\n", dst, nodeVal)
		if m.tempTypes == nil {
			m.tempTypes = make(map[string]string)
		}
		m.tempTypes[strings.TrimPrefix(dst, "%")] = "ptr"
		return
	}

	// JSON array len: __json_array_len(node) -> i32
	if c.Fn == "__json_array_len" && len(c.Args) == 1 {
		m.ensureDecl("declare i32 @__json_array_len(ptr)")
		dst := c.Dst.Name
		_, nodeVal := m.operand(c.Args[0])
		wprintf(&m.funcs, "  %s = call i32 @__json_array_len(ptr %s)\n", dst, nodeVal)
		m.tempTypes[strings.TrimPrefix(dst, "%")] = "i32"
		return
	}

	// JSON array get: __json_array_get(node, index) -> ptr
	if c.Fn == "__json_array_get" && len(c.Args) == 2 {
		m.ensureDecl("declare ptr @__json_array_get(ptr, i32)")
		dst := c.Dst.Name
		_, nodeVal := m.operand(c.Args[0])
		idxTy, idxVal := m.operand(c.Args[1])
		// Ensure index is i32
		if idxTy == "i64" {
			truncIdx := fmt.Sprintf("%%t%d", m.tempID)
			m.tempID++
			wprintf(&m.funcs, "  %s = trunc i64 %s to i32\n", truncIdx, idxVal)
			idxVal = truncIdx
		}
		wprintf(&m.funcs, "  %s = call ptr @__json_array_get(ptr %s, i32 %s)\n", dst, nodeVal, idxVal)
		if m.tempTypes == nil {
			m.tempTypes = make(map[string]string)
		}
		m.tempTypes[strings.TrimPrefix(dst, "%")] = "ptr"
		return
	}

	// JSON object len: __json_object_len(node) -> i32
	if c.Fn == "__json_object_len" && len(c.Args) == 1 {
		m.ensureDecl("declare i32 @__json_object_len(ptr)")
		dst := c.Dst.Name
		_, nodeVal := m.operand(c.Args[0])
		wprintf(&m.funcs, "  %s = call i32 @__json_object_len(ptr %s)\n", dst, nodeVal)
		m.tempTypes[strings.TrimPrefix(dst, "%")] = "i32"
		return
	}

	// JSON object get: __json_object_get(node, key) -> ptr
	if c.Fn == "__json_object_get" && len(c.Args) == 2 {
		m.ensureDecl("declare ptr @__json_object_get(ptr, ptr)")
		dst := c.Dst.Name
		_, nodeVal := m.operand(c.Args[0])
		_, keyVal := m.operand(c.Args[1])
		wprintf(&m.funcs, "  %s = call ptr @__json_object_get(ptr %s, ptr %s)\n", dst, nodeVal, keyVal)
		if m.tempTypes == nil {
			m.tempTypes = make(map[string]string)
		}
		m.tempTypes[strings.TrimPrefix(dst, "%")] = "ptr"
		return
	}

	// Math module: __math_is_nan(double) -> i1
	if c.Fn == "__math_is_nan" && len(c.Args) == 1 {
		m.ensureDecl("declare i1 @__math_is_nan(double)")
		dst := c.Dst.Name
		_, val := m.operand(c.Args[0])
		wprintf(&m.funcs, "  %s = call i1 @__math_is_nan(double %s)\n", dst, val)
		if m.tempTypes == nil {
			m.tempTypes = make(map[string]string)
		}
		m.tempTypes[strings.TrimPrefix(dst, "%")] = "i1"
		return
	}

	// Math module: __math_is_inf(double) -> i1
	if c.Fn == "__math_is_inf" && len(c.Args) == 1 {
		m.ensureDecl("declare i1 @__math_is_inf(double)")
		dst := c.Dst.Name
		_, val := m.operand(c.Args[0])
		wprintf(&m.funcs, "  %s = call i1 @__math_is_inf(double %s)\n", dst, val)
		if m.tempTypes == nil {
			m.tempTypes = make(map[string]string)
		}
		m.tempTypes[strings.TrimPrefix(dst, "%")] = "i1"
		return
	}

	// Math module: __math_is_finite(double) -> i1
	if c.Fn == "__math_is_finite" && len(c.Args) == 1 {
		m.ensureDecl("declare i1 @__math_is_finite(double)")
		dst := c.Dst.Name
		_, val := m.operand(c.Args[0])
		wprintf(&m.funcs, "  %s = call i1 @__math_is_finite(double %s)\n", dst, val)
		if m.tempTypes == nil {
			m.tempTypes = make(map[string]string)
		}
		m.tempTypes[strings.TrimPrefix(dst, "%")] = "i1"
		return
	}

	// Built-in print via puts (strings) or print_int (integers)
	if c.Fn == "print" && len(c.Args) == 1 {
		// Special case for string literal
		if s, ok := c.Args[0].(hir.ConstStr); ok {
			// Use printf("%s\n", ...) instead of puts() to handle strings with explicit newlines
			// This way print("hello\n") outputs "hello\n\n" (explicit newline + print's newline)
			m.ensureDecl("declare i32 @printf(ptr, ...)")
			fmtG, fmtN := m.ensureCStringGlobal("%s\n", false)
			wprintf(&m.funcs, "  %%t%d = getelementptr inbounds [%d x i8], [%d x i8]* %s, i64 0, i64 0\n",
				m.tempID, fmtN, fmtN, fmtG)
			strG, strN := m.ensureCStringGlobal(s.Text, false)
			wprintf(&m.funcs, "  %%t%d = getelementptr inbounds [%d x i8], [%d x i8]* %s, i64 0, i64 0\n",
				m.tempID+1, strN, strN, strG)
			wprintf(&m.funcs, "  %%t%d = call i32 (ptr, ...) @printf(ptr %%t%d, ptr %%t%d)\n",
				m.tempID+2, m.tempID, m.tempID+1)
			m.tempID += 3
			return
		}
		// Integer arguments: call print_int
		ty, val := m.operand(c.Args[0])
		if ty == "i32" || ty == "i64" {
			m.ensureDecl("declare void @print_int(i64)")
			// Cast to i64 if needed
			if ty == "i32" {
				wprintf(&m.funcs, "  %%t%d = sext i32 %s to i64\n", m.tempID, val)
				wprintf(&m.funcs, "  call void @print_int(i64 %%t%d)\n", m.tempID)
				m.tempID++
			} else {
				wprintf(&m.funcs, "  call void @print_int(i64 %s)\n", val)
			}
			return
		}
		// Float arguments: call print_float_py (Python-like formatting with .0 for whole numbers)
		if ty == "double" {
			m.ensureDecl("declare void @print_float_py(double)")
			wprintf(&m.funcs, "  call void @print_float_py(double %s)\n", val)
			return
		}
		// Boolean arguments: print true/false
		if ty == "i1" {
			m.ensureDecl("declare i32 @printf(ptr, ...)")
			// Select string based on value
			// Inline implementation with select:
			// %str = select i1 %val, ptr @true_str, ptr @false_str
			// call printf("%s\n", %str)

			trueG, trueN := m.ensureCStringGlobal("true", false)
			falseG, falseN := m.ensureCStringGlobal("false", false)

			wprintf(&m.funcs, "  %%t%d = getelementptr inbounds [%d x i8], [%d x i8]* %s, i64 0, i64 0\n", m.tempID, trueN, trueN, trueG)
			truePtr := fmt.Sprintf("%%t%d", m.tempID)
			m.tempID++

			wprintf(&m.funcs, "  %%t%d = getelementptr inbounds [%d x i8], [%d x i8]* %s, i64 0, i64 0\n", m.tempID, falseN, falseN, falseG)
			falsePtr := fmt.Sprintf("%%t%d", m.tempID)
			m.tempID++

			wprintf(&m.funcs, "  %%t%d = select i1 %s, ptr %s, ptr %s\n", m.tempID, val, truePtr, falsePtr)
			selPtr := fmt.Sprintf("%%t%d", m.tempID)
			m.tempID++

			// Use printf("%s\n", ...) to print the selected string
			fmtG, fmtN := m.ensureCStringGlobal("%s\n", false)
			wprintf(&m.funcs, "  %%t%d = getelementptr inbounds [%d x i8], [%d x i8]* %s, i64 0, i64 0\n", m.tempID, fmtN, fmtN, fmtG)
			fmtPtr := fmt.Sprintf("%%t%d", m.tempID)
			m.tempID++

			wprintf(&m.funcs, "  %%t%d = call i32 (ptr, ...) @printf(ptr %s, ptr %s)\n", m.tempID, fmtPtr, selPtr)
			m.tempID++
			return
		}

		// General case: if arg is ptr, assume it's a string and call printf
		if ty == "ptr" {
			m.ensureDecl("declare i32 @printf(ptr, ...)")
			// Use printf("%s\n", str) to preserve explicit newlines in the string
			fmtG, fmtN := m.ensureCStringGlobal("%s\n", false)
			wprintf(&m.funcs, "  %%t%d = getelementptr inbounds [%d x i8], [%d x i8]* %s, i64 0, i64 0\n",
				m.tempID, fmtN, fmtN, fmtG)
			wprintf(&m.funcs, "  %%t%d = call i32 (ptr, ...) @printf(ptr %%t%d, ptr %s)\n",
				m.tempID+1, m.tempID, val)
			m.tempID += 2
			return
		}
	}

	// Built-in print_item - print without newline (for multi-arg print)
	if c.Fn == "print_item" && len(c.Args) == 1 {
		// Special case for string literal
		if s, ok := c.Args[0].(hir.ConstStr); ok {
			m.ensureDecl("declare i32 @printf(ptr, ...)")
			fmtG, fmtN := m.ensureCStringGlobal("%s", false)
			wprintf(&m.funcs, "  %%t%d = getelementptr inbounds [%d x i8], [%d x i8]* %s, i64 0, i64 0\n",
				m.tempID, fmtN, fmtN, fmtG)
			strG, strN := m.ensureCStringGlobal(s.Text, false)
			wprintf(&m.funcs, "  %%t%d = getelementptr inbounds [%d x i8], [%d x i8]* %s, i64 0, i64 0\n",
				m.tempID+1, strN, strN, strG)
			wprintf(&m.funcs, "  %%t%d = call i32 (ptr, ...) @printf(ptr %%t%d, ptr %%t%d)\n",
				m.tempID+2, m.tempID, m.tempID+1)
			m.tempID += 3
			return
		}
		// Integer arguments
		ty, val := m.operand(c.Args[0])
		if ty == "i32" || ty == "i64" {
			m.ensureDecl("declare i32 @printf(ptr, ...)")
			fmtG, fmtN := m.ensureCStringGlobal("%ld", false)
			wprintf(&m.funcs, "  %%t%d = getelementptr inbounds [%d x i8], [%d x i8]* %s, i64 0, i64 0\n",
				m.tempID, fmtN, fmtN, fmtG)
			fmtPtr := fmt.Sprintf("%%t%d", m.tempID)
			m.tempID++
			if ty == "i32" {
				wprintf(&m.funcs, "  %%t%d = sext i32 %s to i64\n", m.tempID, val)
				wprintf(&m.funcs, "  %%t%d = call i32 (ptr, ...) @printf(ptr %s, i64 %%t%d)\n", m.tempID+1, fmtPtr, m.tempID)
				m.tempID += 2
			} else {
				wprintf(&m.funcs, "  %%t%d = call i32 (ptr, ...) @printf(ptr %s, i64 %s)\n", m.tempID, fmtPtr, val)
				m.tempID++
			}
			return
		}
		// Float arguments: call print_float_item_py (Python-like, no newline)
		if ty == "double" {
			m.ensureDecl("declare void @print_float_item_py(double)")
			wprintf(&m.funcs, "  call void @print_float_item_py(double %s)\n", val)
			return
		}
		// Boolean arguments
		if ty == "i1" {
			m.ensureDecl("declare i32 @printf(ptr, ...)")
			trueG, trueN := m.ensureCStringGlobal("true", false)
			falseG, falseN := m.ensureCStringGlobal("false", false)
			wprintf(&m.funcs, "  %%t%d = getelementptr inbounds [%d x i8], [%d x i8]* %s, i64 0, i64 0\n",
				m.tempID, trueN, trueN, trueG)
			truePtr := fmt.Sprintf("%%t%d", m.tempID)
			m.tempID++
			wprintf(&m.funcs, "  %%t%d = getelementptr inbounds [%d x i8], [%d x i8]* %s, i64 0, i64 0\n",
				m.tempID, falseN, falseN, falseG)
			falsePtr := fmt.Sprintf("%%t%d", m.tempID)
			m.tempID++
			wprintf(&m.funcs, "  %%t%d = select i1 %s, ptr %s, ptr %s\n", m.tempID, val, truePtr, falsePtr)
			selPtr := fmt.Sprintf("%%t%d", m.tempID)
			m.tempID++
			fmtG, fmtN := m.ensureCStringGlobal("%s", false)
			wprintf(&m.funcs, "  %%t%d = getelementptr inbounds [%d x i8], [%d x i8]* %s, i64 0, i64 0\n",
				m.tempID, fmtN, fmtN, fmtG)
			fmtPtr := fmt.Sprintf("%%t%d", m.tempID)
			m.tempID++
			wprintf(&m.funcs, "  %%t%d = call i32 (ptr, ...) @printf(ptr %s, ptr %s)\n", m.tempID, fmtPtr, selPtr)
			m.tempID++
			return
		}
		// General case: ptr (string)
		if ty == "ptr" {
			m.ensureDecl("declare i32 @printf(ptr, ...)")
			fmtG, fmtN := m.ensureCStringGlobal("%s", false)
			wprintf(&m.funcs, "  %%t%d = getelementptr inbounds [%d x i8], [%d x i8]* %s, i64 0, i64 0\n",
				m.tempID, fmtN, fmtN, fmtG)
			wprintf(&m.funcs, "  %%t%d = call i32 (ptr, ...) @printf(ptr %%t%d, ptr %s)\n",
				m.tempID+1, m.tempID, val)
			m.tempID += 2
			return
		}
	}

	// Built-in print_raw - print exact string without formatting (for sep/end)
	if c.Fn == "print_raw" && len(c.Args) == 1 {
		if s, ok := c.Args[0].(hir.ConstStr); ok {
			m.ensureDecl("declare i32 @printf(ptr, ...)")
			fmtG, fmtN := m.ensureCStringGlobal("%s", false)
			wprintf(&m.funcs, "  %%t%d = getelementptr inbounds [%d x i8], [%d x i8]* %s, i64 0, i64 0\n",
				m.tempID, fmtN, fmtN, fmtG)
			strG, strN := m.ensureCStringGlobal(s.Text, false)
			wprintf(&m.funcs, "  %%t%d = getelementptr inbounds [%d x i8], [%d x i8]* %s, i64 0, i64 0\n",
				m.tempID+1, strN, strN, strG)
			wprintf(&m.funcs, "  %%t%d = call i32 (ptr, ...) @printf(ptr %%t%d, ptr %%t%d)\n",
				m.tempID+2, m.tempID, m.tempID+1)
			m.tempID += 3
			return
		}
		// Variable string
		ty, val := m.operand(c.Args[0])
		if ty == "ptr" {
			m.ensureDecl("declare i32 @printf(ptr, ...)")
			fmtG, fmtN := m.ensureCStringGlobal("%s", false)
			wprintf(&m.funcs, "  %%t%d = getelementptr inbounds [%d x i8], [%d x i8]* %s, i64 0, i64 0\n",
				m.tempID, fmtN, fmtN, fmtG)
			wprintf(&m.funcs, "  %%t%d = call i32 (ptr, ...) @printf(ptr %%t%d, ptr %s)\n",
				m.tempID+1, m.tempID, val)
			m.tempID += 2
			return
		}
	}

	// Built-in fflush_stdout - flush stdout buffer
	if c.Fn == "fflush_stdout" && len(c.Args) == 0 {
		m.ensureDecl("declare i32 @fflush(ptr)")
		// fflush(stdout) - pass null for stdout
		wprintf(&m.funcs, "  %%t%d = call i32 @fflush(ptr null)\n", m.tempID)
		m.tempID++
		return
	}

	// Built-in fflush_stream - flush specific stream (1=stdout, 2=stderr)
	if c.Fn == "fflush_stream" && len(c.Args) == 1 {
		m.ensureDecl("declare i32 @fflush(ptr)")
		m.ensureDecl("@stderr = external global ptr")
		if constInt, ok := c.Args[0].(*hir.ConstInt); ok && constInt.Text == "2" {
			// stderr
			wprintf(&m.funcs, "  %%t%d = load ptr, ptr @stderr\n", m.tempID)
			wprintf(&m.funcs, "  %%t%d = call i32 @fflush(ptr %%t%d)\n", m.tempID+1, m.tempID)
			m.tempID += 2
		} else {
			// stdout (null)
			wprintf(&m.funcs, "  %%t%d = call i32 @fflush(ptr null)\n", m.tempID)
			m.tempID++
		}
		return
	}

	// Built-in print_raw_stream - print string to specific stream
	if c.Fn == "print_raw_stream" && len(c.Args) == 2 {
		m.ensureDecl("declare i32 @fputs(ptr, ptr)")
		m.ensureDecl("@stderr = external global ptr")

		streamID := "1"
		if constInt, ok := c.Args[0].(*hir.ConstInt); ok {
			streamID = constInt.Text
		}

		if s, ok := c.Args[1].(hir.ConstStr); ok {
			strG, strN := m.ensureCStringGlobal(s.Text, false)
			wprintf(&m.funcs, "  %%t%d = getelementptr inbounds [%d x i8], [%d x i8]* %s, i64 0, i64 0\n",
				m.tempID, strN, strN, strG)
			if streamID == "2" {
				wprintf(&m.funcs, "  %%t%d = load ptr, ptr @stderr\n", m.tempID+1)
				wprintf(&m.funcs, "  %%t%d = call i32 @fputs(ptr %%t%d, ptr %%t%d)\n",
					m.tempID+2, m.tempID, m.tempID+1)
				m.tempID += 3
			} else {
				// stdout (null means stdout in some systems, but use fputs with stdout)
				wprintf(&m.funcs, "  %%t%d = call i32 (ptr, ...) @printf(ptr %%t%d)\n",
					m.tempID+1, m.tempID)
				m.tempID += 2
			}
			return
		}
	}

	// Built-in str() conversion
	if c.Fn == "str" && len(c.Args) == 1 {
		ty, val := m.operand(c.Args[0])

		// Determine destination name
		dst := ""
		if c.Dst.Name != "" {
			dst = c.Dst.String()
		} else {
			dst = fmt.Sprintf("%%t%d", m.tempID)
			m.tempID++
		}

		// Helper to register type
		registerType := func(name string) {
			if strings.HasPrefix(name, "%") {
				name = name[1:]
			}
			m.tempTypes[name] = "ptr"
			m.varTypes[name] = types.Str
		}

		// Integer to string
		if ty == "i32" || ty == "i64" {
			m.ensureDecl("declare ptr @int_to_str(i32)")
			if ty == "i64" {
				// Truncate to i32 for now
				truncDst := fmt.Sprintf("%%t%d", m.tempID)
				m.tempID++
				wprintf(&m.funcs, "  %s = trunc i64 %s to i32\n", truncDst, val)
				val = truncDst
			}
			wprintf(&m.funcs, "  %s = call ptr @int_to_str(i32 %s)\n", dst, val)
			registerType(dst)
			return
		}

		// Float to string
		if ty == "double" || ty == "float" {
			m.ensureDecl("declare ptr @float_to_str(double)")
			if ty == "float" {
				extDst := fmt.Sprintf("%%t%d", m.tempID)
				m.tempID++
				wprintf(&m.funcs, "  %s = fpext float %s to double\n", extDst, val)
				val = extDst
			}
			wprintf(&m.funcs, "  %s = call ptr @float_to_str(double %s)\n", dst, val)
			registerType(dst)
			return
		}

		// Bool to string
		if ty == "i1" {
			m.ensureDecl("declare ptr @bool_to_str(i32)")
			extTemp := fmt.Sprintf("%%bext_%d", m.tempID)
			m.tempID++
			wprintf(&m.funcs, "  %s = zext i1 %s to i32\n", extTemp, val)
			wprintf(&m.funcs, "  %s = call ptr @bool_to_str(i32 %s)\n", dst, extTemp)
			registerType(dst)
			return
		}

		// String to string (identity)
		if ty == "ptr" {
			// Check if it's a string or struct
			getType := func(val hir.Value) types.T {
				if v, ok := val.(hir.Var); ok {
					return m.varTypes[v.Name]
				}
				if t, ok := val.(hir.Temp); ok {
					return m.varTypes[t.Name]
				}
				return nil
			}

			argType := getType(c.Args[0])
			isStr := false
			if argType != nil && types.Equal(argType, types.Str) {
				isStr = true
			} else if _, ok := c.Args[0].(hir.ConstStr); ok {
				isStr = true
			}

			if isStr {
				// Identity
				wprintf(&m.funcs, "  %s = bitcast ptr %s to ptr\n", dst, val)
				registerType(dst)
				return
			}

			// Struct to string
			typeName := "Unknown"
			if argType != nil {
				if s, ok := argType.(*types.Struct); ok {
					typeName = s.Name
				} else if cls, ok := argType.(*types.Class); ok {
					typeName = cls.Name
				}
			}

			wprintf(&m.funcs, "  %s = call ptr @%s_to_str(ptr %s)\n", dst, typeName, val)
			m.ensureDecl(fmt.Sprintf("declare ptr @%s_to_str(ptr)", typeName))
			registerType(dst)
			return
		}
	}

	// Built-in __assert_check(cond, msg) - conditional abort
	if c.Fn == "__assert_check" && len(c.Args) == 2 {
		// Get condition
		condTy, condVal := m.operand(c.Args[0])

		// Get message
		_, msgVal := m.operand(c.Args[1])

		// Generate unique labels
		okLabel := fmt.Sprintf("assert_ok%d", m.mergeID)
		failLabel := fmt.Sprintf("assert_fail%d", m.mergeID)
		m.mergeID++

		// Emit conditional branch: if cond goto ok else goto fail
		wprintf(&m.funcs, "  br %s %s, label %%%s, label %%%s\n", condTy, condVal, okLabel, failLabel)

		// Emit fail block: print message and abort
		wprintf(&m.funcs, "%s:\n", failLabel)
		m.ensureDecl("declare i32 @printf(ptr, ...)")
		m.ensureDecl("declare void @exit(i32)")

		// Print error message
		fmtG, fmtN := m.ensureCStringGlobal("Assertion failed: %s\n", false)
		wprintf(&m.funcs, "  %%t%d = getelementptr inbounds [%d x i8], [%d x i8]* %s, i64 0, i64 0\n",
			m.tempID, fmtN, fmtN, fmtG)
		wprintf(&m.funcs, "  %%t%d = call i32 (ptr, ...) @printf(ptr %%t%d, ptr %s)\n",
			m.tempID+1, m.tempID, msgVal)
		m.tempID += 2

		// Call exit(1)
		wprintf(&m.funcs, "  call void @exit(i32 1)\n")
		wprintf(&m.funcs, "  unreachable\n")

		// Emit ok block: continue
		wprintf(&m.funcs, "%s:\n", okLabel)
		return
	}
	// __future_register_poll(fut, &name$poll, frame)
	if c.Fn == "__future_register_poll" {
		m.ensureDecl("declare void @__future_register_poll(ptr, ptr, ptr)")
		futOp := "ptr null"
		if len(c.Args) > 0 {
			futOp = m.ptrOperand(c.Args[0])
		}
		fnOp := "ptr null"
		if len(c.Args) > 1 {
			if v, ok := c.Args[1].(hir.Var); ok && strings.HasPrefix(v.Name, "&") {
				fnOp = "ptr @" + v.Name[1:]
				if strings.HasSuffix(v.Name, "$poll") {
					base := strings.TrimSuffix(v.Name[1:], "$poll")
					m.MarkAsyncWrapper(base)
				}
			} else {
				fnOp = m.ptrOperand(c.Args[1])
			}
		}
		frameOp := "ptr null"
		if len(c.Args) > 2 {
			frameOp = m.ptrOperand(c.Args[2])
		}
		wprintf(&m.funcs, "  call void @__future_register_poll(%s, %s, %s)\n", futOp, fnOp, frameOp)
		return
	}

	// free(ptr): use the canonical typed declare. The generic fallback below
	// would emit `declare i32 @free(...)`, which clashes with the
	// `declare void @free(ptr)` the drop paths emit in the same module —
	// clang rejects two differently-typed declares for one symbol.
	if c.Fn == "free" && len(c.Args) == 1 {
		wprintf(&m.funcs, "  call void @free(%s)\n", m.ptrOperand(c.Args[0]))
		m.ensureDecl("declare void @free(ptr)")
		return
	}

	// Fallback: external call — choose return type via overrides/async, honor c.Dst if provided.
	ret := c.Type
	if ret == "" {
		ret = m.callRetType(c.Fn)
	}

	// Build args
	var argStr strings.Builder
	argStr.WriteString("(")
	for i, a := range c.Args {
		if i > 0 {
			argStr.WriteString(", ")
		}
		ty, val := m.operand(a)

		// Use ABI layer to determine if i32 promotion is needed
		abiInfo := abi.Current()
		if abiInfo.NeedsI32ToI64Promotion(c.Fn) && ty == "i32" {
			wprintf(&m.funcs, "  %%t%d = sext i32 %s to i64\n", m.tempID, val)
			val = fmt.Sprintf("%%t%d", m.tempID)
			m.tempID++
			ty = "i64"
		}

		argStr.WriteString(fmt.Sprintf("%s %s", ty, val))
	}
	argStr.WriteString(")")

	// Ensure we have a declare stub. Use ABI layer for variadic signatures.
	// But skip if this function is defined in the same module
	if !m.definedFunctions[c.Fn] {
		abiInfo := abi.Current()
		sig := abiInfo.VariadicSignature(c.Fn, ret)
		if sig != "" {
			m.ensureDecl(sig)
		} else {
			// Generic variadic declaration
			m.ensureDecl(fmt.Sprintf("declare %s @%s(...)", ret, c.Fn))
		}
	}

	if c.Dst.Name != "" && ret != "void" {
		dst := c.Dst.String()
		// Use ABI layer to get explicit call syntax if needed
		abiInfo := abi.Current()
		explicitSig := abiInfo.ExplicitCallSyntax(c.Fn, ret)
		if explicitSig != "" {
			wprintf(&m.funcs, "  %s = call %s %s @%s%s\n", dst, ret, explicitSig, c.Fn, argStr.String())
		} else {
			wprintf(&m.funcs, "  %s = call %s @%s%s\n", dst, ret, c.Fn, argStr.String())
		}

		if strings.HasPrefix(dst, "%") {
			m.tempTypes[dst] = ret
			// Register high-level return type if available
			if m.info != nil {
				if set, ok := m.info.Funcs[c.Fn]; ok && len(set.Cands) > 0 {
					dstName := dst
					if strings.HasPrefix(dstName, "%") {
						dstName = dstName[1:]
					}
					m.varTypes[dstName] = set.Cands[0].Type.Ret
				}
			}
		}
		if strings.HasPrefix(dst, "%t") {
			if n, err := strconv.Atoi(dst[2:]); err == nil {
				if n >= m.tempID {
					m.tempID = n + 1
				}
			}
		}
		return
	}

	if ret == "void" {
		// Use ABI layer to get explicit call syntax if needed
		abiInfo := abi.Current()
		explicitSig := abiInfo.ExplicitCallSyntax(c.Fn, ret)
		if explicitSig != "" {
			wprintf(&m.funcs, "  call %s %s @%s%s\n", ret, explicitSig, c.Fn, argStr.String())
		} else {
			wprintf(&m.funcs, "  call %s @%s%s\n", ret, c.Fn, argStr.String())
		}
		return
	}

	// Use ABI layer to get explicit call syntax if needed
	abiInfo := abi.Current()
	explicitSig := abiInfo.ExplicitCallSyntax(c.Fn, ret)
	if explicitSig != "" {
		wprintf(&m.funcs, "  %%t%d = call %s %s @%s%s\n", m.tempID, ret, explicitSig, c.Fn, argStr.String())
	} else {
		wprintf(&m.funcs, "  %%t%d = call %s @%s%s\n", m.tempID, ret, c.Fn, argStr.String())
	}
	tempName := fmt.Sprintf("t%d", m.tempID)
	m.tempTypes[tempName] = ret

	// Register high-level return type if available
	if m.info != nil {
		if set, ok := m.info.Funcs[c.Fn]; ok && len(set.Cands) > 0 {
			// Use the return type of the first candidate (sufficient for builtins like str)
			// TODO: Handle overloads with different return types if that ever happens
			m.varTypes[tempName] = set.Cands[0].Type.Ret
		}
	}

	m.tempID++
}
