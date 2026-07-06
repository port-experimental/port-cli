package render

// Format identifies CLI result output encoding.
type Format string

const (
	FormatText Format = "text"
	FormatJSON Format = "json"
	FormatHTML Format = "html"
)
