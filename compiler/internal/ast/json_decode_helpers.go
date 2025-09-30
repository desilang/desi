package ast

func asMap(v any) (map[string]any, bool) {
  m, ok := v.(map[string]any)
  return m, ok
}
func getString(m map[string]any, k string) string {
  if v, ok := m[k]; ok {
    if s, ok := v.(string); ok {
      return s
    }
  }
  return ""
}
func getBool(m map[string]any, k string) bool {
  if v, ok := m[k]; ok {
    if b, ok := v.(bool); ok {
      return b
    }
  }
  return false
}
func getSlice(m map[string]any, k string) []any {
  if v, ok := m[k]; ok {
    if a, ok := v.([]any); ok {
      return a
    }
  }
  return nil
}
func getMap(m map[string]any, k string) map[string]any {
  if v, ok := m[k]; ok {
    if mm, ok := v.(map[string]any); ok {
      return mm
    }
  }
  return nil
}
func getIntFromAny(v any) int {
  switch t := v.(type) {
  case float64:
    return int(t)
  case int:
    return t
  default:
    return 0
  }
}
func parseSpan(mm map[string]any) Span {
  if mm == nil {
    return Span{}
  }
  st := getMap(mm, "start")
  en := getMap(mm, "end")
  return Span{
    Start: Pos{Line: getIntFromAny(st["line"]), Col: getIntFromAny(st["col"])},
    End:   Pos{Line: getIntFromAny(en["line"]), Col: getIntFromAny(en["col"])},
  }
}
