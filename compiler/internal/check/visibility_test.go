package check

import (
  "testing"

  "github.com/desilang/desi/compiler/internal/ast"
)

// helper to collect error codes from []error (via diag-backed error strings)
func hasErrSubstr(errs []error, want string) bool {
  for _, e := range errs {
    if e == nil {
      continue
    }
    if strstr(e.Error(), want) {
      return true
    }
  }
  return false
}

func strstr(hay, needle string) bool {
  return len(needle) == 0 || (len(hay) >= len(needle) && (func() bool {
    for i := 0; i+len(needle) <= len(hay); i++ {
      if hay[i:i+len(needle)] == needle {
        return true
      }
    }
    return false
  })())
}

// --- Cases ---
//
// We synthesize a merged file that contains:
//   • provider module decls (pretend they belong to util.math)
//   • consumer code that imports/calls/uses those symbols
//
// Visibility is enforced by the checker via isPublic* and the Phase B rule in
// checkfile.go. All errors should be DTE0010 (“symbol is not public”).

// Non-public function via module alias call -> DTE0010
func TestVisibility_ModuleAlias_Call_NonPublicFunc(t *testing.T) {
  f := &ast.File{
    // Provider: non-public function "add"
    Decls: []ast.Decl{
      &ast.FuncDecl{
        Name: "add", Pub: false,
        Params: []ast.Param{
          {Name: "a", Type: "int"},
          {Name: "b", Type: "int"},
        },
        Ret: "int",
        Body: []ast.Stmt{
          &ast.ReturnStmt{Expr: &ast.IntLit{Value: 0}},
        },
      },
      // Consumer: import util.math as m
      &ast.ImportDecl{Path: "util.math", As: "m"},
      // Consumer: main calls m.add(1,2)
      &ast.FuncDecl{
        Name: "main", Pub: false, Ret: "int",
        Body: []ast.Stmt{
          &ast.ExprStmt{
            Expr: &ast.CallExpr{
              Callee: &ast.FieldExpr{
                X:    &ast.IdentExpr{Name: "m"},
                Name: "add",
              },
              Args: []ast.Expr{&ast.IntLit{Value: 1}, &ast.IntLit{Value: 2}},
            },
          },
          &ast.ReturnStmt{Expr: &ast.IntLit{Value: 0}},
        },
      },
    },
  }
  _, errs, _ := CheckFile(f)
  if !hasErrSubstr(errs, "DTE0010") {
    t.Fatalf("expected DTE0010 (not public) on m.add call; got: %#v", errs)
  }
}

// Public function via module alias call -> OK (no DTE0010)
func TestVisibility_ModuleAlias_Call_PublicFunc_OK(t *testing.T) {
  f := &ast.File{
    Decls: []ast.Decl{
      // Provider: public function "add"
      &ast.FuncDecl{
        Name: "add", Pub: true,
        Params: []ast.Param{
          {Name: "a", Type: "int"},
          {Name: "b", Type: "int"},
        },
        Ret: "int",
        Body: []ast.Stmt{
          &ast.ReturnStmt{Expr: &ast.IntLit{Value: 0}},
        },
      },
      // Consumer: import util.math as m
      &ast.ImportDecl{Path: "util.math", As: "m"},
      // Consumer: main calls m.add(1,2)
      &ast.FuncDecl{
        Name: "main", Pub: false, Ret: "int",
        Body: []ast.Stmt{
          &ast.ExprStmt{
            Expr: &ast.CallExpr{
              Callee: &ast.FieldExpr{
                X:    &ast.IdentExpr{Name: "m"},
                Name: "add",
              },
              Args: []ast.Expr{&ast.IntLit{Value: 1}, &ast.IntLit{Value: 2}},
            },
          },
          &ast.ReturnStmt{Expr: &ast.IntLit{Value: 0}},
        },
      },
    },
  }
  _, errs, _ := CheckFile(f)
  if hasErrSubstr(errs, "DTE0010") {
    t.Fatalf("did not expect DTE0010 for public function; got: %#v", errs)
  }
}

// Non-public constant via module alias access -> DTE0010
func TestVisibility_ModuleAlias_Const_NonPublic(t *testing.T) {
  f := &ast.File{
    Decls: []ast.Decl{
      // Provider: non-public const VALUE (literal so it’s a valid const)
      &ast.ConstDecl{Name: "VALUE", Pub: false, Value: &ast.IntLit{Value: 42}},
      // Consumer: import util.math as m
      &ast.ImportDecl{Path: "util.math", As: "m"},
      // Consumer: let x = m.VALUE
      &ast.FuncDecl{
        Name: "main", Ret: "int",
        Body: []ast.Stmt{
          &ast.LetStmt{
            Name: "x",
            Expr: &ast.FieldExpr{
              X:    &ast.IdentExpr{Name: "m"},
              Name: "VALUE",
            },
          },
          &ast.ReturnStmt{Expr: &ast.IntLit{Value: 0}},
        },
      },
    },
  }
  _, errs, _ := CheckFile(f)
  if !hasErrSubstr(errs, "DTE0010") {
    t.Fatalf("expected DTE0010 for non-public module const; got: %#v", errs)
  }
}

// Public constant via module alias access -> OK
func TestVisibility_ModuleAlias_Const_Public_OK(t *testing.T) {
  f := &ast.File{
    Decls: []ast.Decl{
      &ast.ConstDecl{Name: "VALUE", Pub: true, Value: &ast.IntLit{Value: 42}},
      &ast.ImportDecl{Path: "util.math", As: "m"},
      &ast.FuncDecl{
        Name: "main", Ret: "int",
        Body: []ast.Stmt{
          &ast.LetStmt{
            Name: "x",
            Expr: &ast.FieldExpr{
              X:    &ast.IdentExpr{Name: "m"},
              Name: "VALUE",
            },
          },
          &ast.ReturnStmt{Expr: &ast.IntLit{Value: 0}},
        },
      },
    },
  }
  _, errs, _ := CheckFile(f)
  if hasErrSubstr(errs, "DTE0010") {
    t.Fatalf("did not expect DTE0010 for public module const; got: %#v", errs)
  }
}

// From-import item must be public -> DTE0010 at import time
func TestVisibility_FromImport_Item_MustBePublic(t *testing.T) {
  f := &ast.File{
    Decls: []ast.Decl{
      // Provider: non-public function "add"
      &ast.FuncDecl{
        Name: "add", Pub: false,
        Params: []ast.Param{{Name: "a", Type: "int"}, {Name: "b", Type: "int"}},
        Ret:    "int",
        Body:   []ast.Stmt{&ast.ReturnStmt{Expr: &ast.IntLit{Value: 0}}},
      },
      // Consumer: from util.math import add
      &ast.FromImportDecl{
        Module: "util.math",
        Items:  []ast.ImportItem{{Name: "add"}},
      },
      // Using it (will fail anyway), but violation is caught at import time
      &ast.FuncDecl{
        Name: "main", Ret: "int",
        Body: []ast.Stmt{
          &ast.ExprStmt{Expr: &ast.CallExpr{Callee: &ast.IdentExpr{Name: "add"}}},
          &ast.ReturnStmt{Expr: &ast.IntLit{Value: 0}},
        },
      },
    },
  }
  _, errs, _ := CheckFile(f)
  if !hasErrSubstr(errs, "DTE0010") {
    t.Fatalf("expected DTE0010 for non-public from-import item; got: %#v", errs)
  }
}

// Sanity: from-import of public item OK
func TestVisibility_FromImport_PublicItem_OK(t *testing.T) {
  f := &ast.File{
    Decls: []ast.Decl{
      &ast.FuncDecl{
        Name: "add", Pub: true,
        Params: []ast.Param{{Name: "a", Type: "int"}, {Name: "b", Type: "int"}},
        Ret:    "int",
        Body:   []ast.Stmt{&ast.ReturnStmt{Expr: &ast.IntLit{Value: 0}}},
      },
      &ast.FromImportDecl{
        Module: "util.math",
        Items:  []ast.ImportItem{{Name: "add"}},
      },
      &ast.FuncDecl{
        Name: "main", Ret: "int",
        Body: []ast.Stmt{
          &ast.ExprStmt{Expr: &ast.CallExpr{Callee: &ast.IdentExpr{Name: "add"}}},
          &ast.ReturnStmt{Expr: &ast.IntLit{Value: 0}},
        },
      },
    },
  }
  _, errs, _ := CheckFile(f)
  if hasErrSubstr(errs, "DTE0010") {
    t.Fatalf("did not expect DTE0010 for public from-import item; got: %#v", errs)
  }
}
