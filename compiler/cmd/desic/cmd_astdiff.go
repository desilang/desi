package main

import (
  "encoding/json"
  "fmt"
  "os"
  "path/filepath"
  "reflect"
  "sort"
  "strings"

  "github.com/desilang/desi/compiler/internal/term"
)

// cmdASTDiff compares two AST JSON files (e.g., from --dump-json).
// Usage:
//
//	desic ast-diff [--ignore-spans] [--ignore-keys=k1,k2,...] <left.json> <right.json>
func cmdASTDiff(args []string) int {
  ignoreSpans := false
  files := []string{}
  var ignoreKeys []string

  usage := func() int {
    term.Eprintln("usage: desic ast-diff [--ignore-spans] [--ignore-keys=k1,k2,...] <left.json> <right.json>")
    return 2
  }

  for _, a := range args {
    switch {
    case a == "--ignore-spans":
      ignoreSpans = true
    case strings.HasPrefix(a, "--ignore-keys="):
      v := strings.TrimPrefix(a, "--ignore-keys=")
      if v != "" {
        for _, k := range strings.Split(v, ",") {
          k = strings.TrimSpace(k)
          if k != "" {
            ignoreKeys = append(ignoreKeys, k)
          }
        }
      }
    default:
      if strings.HasPrefix(a, "-") {
        return usage()
      }
      files = append(files, a)
    }
  }
  if len(files) != 2 {
    return usage()
  }

  left, err := readJSON(files[0])
  if err != nil {
    term.Eprintf("error: read %s: %v\n", files[0], err)
    return 1
  }
  right, err := readJSON(files[1])
  if err != nil {
    term.Eprintf("error: read %s: %v\n", files[1], err)
    return 1
  }

  // Apply ignores
  if ignoreSpans {
    ignoreKeys = append(ignoreKeys, "span", "Span") // be liberal
  }
  if len(ignoreKeys) > 0 {
    pruneKeys(left, ignoreKeys)
    pruneKeys(right, ignoreKeys)
  }

  ok, msg := deepEqualWithPath(left, right)
  if ok {
    suffix := ""
    if ignoreSpans {
      suffix = " [spans ignored]"
    }
    if len(ignoreKeys) > 0 {
      // show unique list without "span"/"Span" duplication noise
      set := map[string]struct{}{}
      for _, k := range ignoreKeys {
        set[k] = struct{}{}
      }
      keys := make([]string, 0, len(set))
      for k := range set {
        keys = append(keys, k)
      }
      sort.Strings(keys)
      if suffix != "" {
        suffix += " "
      }
      suffix += fmt.Sprintf("[ignored: %s]", strings.Join(keys, ","))
    }
    term.Printf("ASTs match (%s == %s)%s\n", short(files[0]), short(files[1]), func() string {
      if suffix != "" {
        return " " + suffix
      }
      return ""
    }())
    return 0
  }

  term.Eprintf("ASTs differ (%s vs %s)%s\n%s\n",
    short(files[0]), short(files[1]),
    func() string {
      if ignoreSpans || len(ignoreKeys) > 0 {
        tag := []string{}
        if ignoreSpans {
          tag = append(tag, "spans ignored")
        }
        if len(ignoreKeys) > 0 {
          tag = append(tag, "ignored keys active")
        }
        return " [" + strings.Join(tag, ", ") + "]"
      }
      return ""
    }(),
    msg,
  )
  return 1
}

/* ---------- helpers ---------- */

func readJSON(path string) (any, error) {
  data, err := os.ReadFile(path)
  if err != nil {
    return nil, err
  }
  var v any
  if err := json.Unmarshal(data, &v); err != nil {
    return nil, err
  }
  return v, nil
}

func short(p string) string {
  if abs, err := filepath.Abs(p); err == nil {
    return abs
  }
  return p
}

// pruneKey removes any map entry named key recursively.
func pruneKey(v any, key string) {
  switch vv := v.(type) {
  case map[string]any:
    delete(vv, key)
    for _, x := range vv {
      pruneKey(x, key)
    }
  case []any:
    for _, x := range vv {
      pruneKey(x, key)
    }
  }
}

// pruneKeys removes any of the provided keys recursively.
func pruneKeys(v any, keys []string) {
  // To avoid repeated walks per key, do a single recursive walk that checks all keys.
  switch vv := v.(type) {
  case map[string]any:
    for _, k := range keys {
      delete(vv, k)
    }
    for _, x := range vv {
      pruneKeys(x, keys)
    }
  case []any:
    for _, x := range vv {
      pruneKeys(x, keys)
    }
  }
}

// deepEqualWithPath returns first mismatch with a breadcrumb path.
func deepEqualWithPath(a, b any) (bool, string) {
  var walk func(a, b any, path []string) (bool, string)
  join := func(path []string) string {
    if len(path) == 0 {
      return "<root>"
    }
    return strings.Join(path, "")
  }
  walk = func(a, b any, path []string) (bool, string) {
    // nil vs non-nil
    if a == nil || b == nil {
      if a == b {
        return true, ""
      }
      return false, fmt.Sprintf("at %s: one is null, the other is not", join(path))
    }

    // normalize numbers (JSON uses float64)
    if af, aok := a.(float64); aok {
      a = af
    }
    if bf, bok := b.(float64); bok {
      b = bf
    }

    ka := reflect.TypeOf(a).Kind()
    kb := reflect.TypeOf(b).Kind()
    if ka != kb {
      return false, fmt.Sprintf("at %s: type mismatch (%s vs %s)", join(path), ka, kb)
    }

    switch ka {
    case reflect.Map:
      ma := a.(map[string]any)
      mb := b.(map[string]any)

      ka := make([]string, 0, len(ma))
      kb := make([]string, 0, len(mb))
      for k := range ma {
        ka = append(ka, k)
      }
      for k := range mb {
        kb = append(kb, k)
      }
      sort.Strings(ka)
      sort.Strings(kb)
      if !reflect.DeepEqual(ka, kb) {
        return false, fmt.Sprintf("at %s: key sets differ\n  left:  %v\n  right: %v", join(path), ka, kb)
      }
      for _, k := range ka {
        ok, msg := walk(ma[k], mb[k], append(path, "."+k))
        if !ok {
          return false, msg
        }
      }
      return true, ""

    case reflect.Slice, reflect.Array:
      sa := a.([]any)
      sb := b.([]any)
      if len(sa) != len(sb) {
        return false, fmt.Sprintf("at %s: length differs (%d vs %d)", join(path), len(sa), len(sb))
      }
      for i := range sa {
        ok, msg := walk(sa[i], sb[i], append(path, fmt.Sprintf("[%d]", i)))
        if !ok {
          return false, msg
        }
      }
      return true, ""

    default:
      if reflect.DeepEqual(a, b) {
        return true, ""
      }
      return false, fmt.Sprintf("at %s: values differ (%v vs %v)", join(path), a, b)
    }
  }
  return walk(a, b, nil)
}
