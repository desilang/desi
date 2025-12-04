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
		// Float arguments: call printf with %f
		if ty == "double" {
			m.ensureDecl("declare i32 @printf(ptr, ...)")
			// Create format string global
			g, n := m.ensureCStringGlobal("%f\n", true)
			wprintf(&m.funcs, "  %%t%d = getelementptr inbounds [%d x i8], [%d x i8]* %s, i64 0, i64 0\n", m.tempID, n, n, g)
			fmtPtr := fmt.Sprintf("%%t%d", m.tempID)
			m.tempID++
			wprintf(&m.funcs, "  %%t%d = call i32 (ptr, ...) @printf(ptr %s, double %s)\n", m.tempID, fmtPtr, val)
			m.tempID++
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
			m.ensureDecl("declare ptr @bool_to_str(i1)")
			wprintf(&m.funcs, "  %s = call ptr @bool_to_str(i1 %s)\n", dst, val)
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
