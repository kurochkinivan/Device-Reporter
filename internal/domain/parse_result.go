package domain

type ParseResult struct {
	Filename string
	Devices  []*Device // filled in case of a success
	Error    error     // filled in case of an error
}
