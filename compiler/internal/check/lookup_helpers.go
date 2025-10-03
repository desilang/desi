package check

func hasConst(info *Info, name string) bool  { _, ok := info.Consts[name]; return ok }
func hasFunc(info *Info, name string) bool   { _, ok := info.Funcs[name]; return ok }
func hasStruct(info *Info, name string) bool { _, ok := info.Structs[name]; return ok }
func hasType(info *Info, name string) bool   { _, ok := info.Types[name]; return ok }
func hasEnum(info *Info, name string) bool   { _, ok := info.Enums[name]; return ok }
