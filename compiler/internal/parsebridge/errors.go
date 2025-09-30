package parsebridge

// ----- errors -----

type ParserSourcesMissingError struct {
	Path string
}

func (e ParserSourcesMissingError) Error() string {
	return "Desi parser source not found at " + e.Path
}
