package check

// ---- Phase B visibility hooks ----

func (c *checker) isPublicFunc(name string) bool  { return c.info != nil && c.info.FuncsPublic[name] }
func (c *checker) isPublicConst(name string) bool { return c.info != nil && c.info.ConstsPublic[name] }
func (c *checker) isPublicStruct(name string) bool {
	return c.info != nil && c.info.StructsPublic[name]
}
