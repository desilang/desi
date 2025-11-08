package parse

// inAsOperator gates whether KW_in is treated as a binary *membership* operator.
// We temporarily turn this OFF while parsing the *target* of `for … in …`
// (both in comprehensions and statement `for`), so the KW_in token remains
// available to the clause parser.
var inAsOperator = true

func withInAsOperator(on bool, fn func()) {
	prev := inAsOperator
	inAsOperator = on
	fn()
	inAsOperator = prev
}
