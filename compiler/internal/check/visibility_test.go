package check

import (
  "strings"
  "testing"

  "github.com/desilang/desi/compiler/internal/ast"
)

func hasErrSubstr(errs []error, want string) bool {
  for _, e := range errs {
    if e == nil {
      continue
    }
    if strings.Contains(e.Error(), want) {
      return true
    }
  }
  return false
}

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
          &ast.ReturnStmt{Expr: &ast.IntLit{}},
        },
      },
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
              Args: []ast.Expr{&ast.IntLit{}, &ast.IntLit{}},
            },
          },
          &ast.ReturnStmt{Expr: &ast.IntLit{}},
        },
      },
    },
    // Consumer: import util.math as m
    Imports: []ast.ImportDecl{
      {Path: "util.math", As: "m"},
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
          &ast.ReturnStmt{Expr: &ast.IntLit{}},
        },
      },
      // Consumer
      &ast.FuncDecl{
        Name: "main", Pub: false, Ret: "int",
        Body: []ast.Stmt{
          &ast.ExprStmt{
            Expr: &ast.CallExpr{
              Callee: &ast.FieldExpr{
                X:    &ast.IdentExpr{Name: "m"},
                Name: "add",
              },
              Args: []ast.Expr{&ast.IntLit{}, &ast.IntLit{}},
            },
          },
          &ast.ReturnStmt{Expr: &ast.IntLit{}},
        },
      },
    },
    Imports: []ast.ImportDecl{
      {Path: "util.math", As: "m"},
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
      &ast.ConstDecl{Name: "VALUE", Pub: false, Value: &ast.IntLit{}},
      // Consumer: let x = m.VALUE
      &ast.FuncDecl{
        Name: "main", Ret: "int",
        Body: []ast.Stmt{
          &ast.LetStmt{
            Binds: []ast.LetBind{{Name: "x"}},
            Values: []ast.Expr{
              &ast.FieldExpr{
                X:    &ast.IdentExpr{Name: "m"},
                Name: "VALUE",
              },
            },
          },
          &ast.ReturnStmt{Expr: &ast.IntLit{}},
        },
      },
    },
    Imports: []ast.ImportDecl{
      {Path: "util.math", As: "m"},
    },
  }
  _, errs, _ := CheckFile(f)
  if !hasErrSubstr(errs, "DTE0010") {
    t.Fatalf("expected DTE0010 for non-public module const; got: %#v", errs)
  }
}

// Public constant via module alias access -> OK
// Public constant via module alias access -> OK
func TestVisibility_ModuleAlias_Const_Public_OK(t *testing.T) {
  f := &ast.File{
    Decls: []ast.Decl{
      &ast.ConstDecl{Name: "VALUE", Pub: true, Value: &ast.IntLit{}},
      &ast.FuncDecl{
        Name: "main", Ret: "int",
        Body: []ast.Stmt{
          &ast.LetStmt{
            Binds: []ast.LetBind{{Name: "x"}},
            Values: []ast.Expr{
              &ast.FieldExpr{
                X:    &ast.IdentExpr{Name: "m"},
                Name: "VALUE",
              },
            },
          },
          &ast.ReturnStmt{Expr: &ast.IntLit{}},
        },
      },
    },
    Imports: []ast.ImportDecl{
      {Path: "util.math", As: "m"},
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
        Body:   []ast.Stmt{&ast.ReturnStmt{Expr: &ast.IntLit{}}},
      },
      // Consumer user code
      &ast.FuncDecl{
        Name: "main", Ret: "int",
        Body: []ast.Stmt{
          &ast.ExprStmt{Expr: &ast.CallExpr{Callee: &ast.IdentExpr{Name: "add"}}},
          &ast.ReturnStmt{Expr: &ast.IntLit{}},
        },
      },
    },
    FromImports: []ast.FromImportDecl{
      {
        Module: "util.math",
        Items:  []ast.ImportItem{{Name: "add"}},
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
        Body:   []ast.Stmt{&ast.ReturnStmt{Expr: &ast.IntLit{}}},
      },
      &ast.FuncDecl{
        Name: "main", Ret: "int",
        Body: []ast.Stmt{
          &ast.ExprStmt{Expr: &ast.CallExpr{Callee: &ast.IdentExpr{Name: "add"}}},
          &ast.ReturnStmt{Expr: &ast.IntLit{}},
        },
      },
    },
    FromImports: []ast.FromImportDecl{
      {
        Module: "util.math",
        Items:  []ast.ImportItem{{Name: "add"}},
      },
    },
  }
  _, errs, _ := CheckFile(f)
  if hasErrSubstr(errs, "DTE0010") {
    t.Fatalf("did not expect DTE0010 for public from-import item; got: %#v", errs)
  }
}
