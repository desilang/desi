package diag

// Code path constants for frequently used type-checker diagnostics.
// These mirror keys in codes.json (e.g., "type.no_matching_overload").
// Note: we currently construct diagnostics directly with code IDs in the checker;
// these constants are here for future use when we switch to builder-based diags.
const (
	CodeTypeNoMatchingOverload = "type.no_matching_overload"
	CodeTypeAmbiguousOverload  = "type.ambiguous_overload"
	CodeTypeBadPipelineFeed    = "type.bad_pipeline_feed"
	CodeTypeInvalidOperand     = "type.invalid_operand_types"
	CodeTypeNotCallable        = "type.not_callable"
)
