package llvm_test

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/backend/llvm"
	"github.com/desilang/desi/compiler/internal/types"
)

func mustType(name string) types.T {
	t, ok := types.FromName(name)
	if !ok {
		return nil
	}
	return t
}

func TestLowerPrimType_BasicsAndM9Types(t *testing.T) {
	tests := []struct {
		t    types.T
		want string
	}{
		{types.Int, "i32"},
		{types.Float, "double"},
		{types.Bool, "i1"},
		{types.Str, "ptr"},
		{types.None, "void"},
		{mustType("usize"), "i64"},
		{mustType("isize"), "i64"},
		{types.CPtrOf(types.Int), "ptr"},
		{types.FutureOf(types.Int), "ptr"},
	}

	for _, tt := range tests {
		if got := llvm.LowerPrimType(tt.t); got != tt.want {
			t.Fatalf("LowerPrimType(%v) = %q, want %q", tt.t, got, tt.want)
		}
	}
}

func TestLowerFuncSignature(t *testing.T) {
	ft := types.FuncOf(
		[]types.T{
			mustType("usize"),
			types.CPtrOf(types.Float),
		},
		mustType("isize"),
	)
	ret, params := llvm.LowerFuncSignature(ft)
	if ret != "i64" {
		t.Fatalf("ret = %q, want i64", ret)
	}
	if len(params) != 2 || params[0] != "i64" || params[1] != "ptr" {
		t.Fatalf("params = %v, want [i64 ptr]", params)
	}
}
