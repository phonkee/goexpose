package response

var (
	// currentKeyFormat holds current key KeyFormat
	currentKeyFormat = SnakeCaseFormat
)

// Format sets KeyFormat to given KeyFormat
func Format(f KeyFormat) {
	currentKeyFormat = f
}

// CurrentFormat returns current key
func CurrentFormat() KeyFormat {
	return currentKeyFormat
}

type KeyFormat struct {
	ErrorKey      string
	MessageKey    string
	ResultKey     string
	ResultSizeKey string
	StatusKey     string
}

var (
	// CamelCaseFormat sets common keys as camel case
	CamelCaseFormat = KeyFormat{
		ErrorKey:      "Error",
		MessageKey:    "Message",
		ResultKey:     "Result",
		ResultSizeKey: "ResultSize",
		StatusKey:     "Status",
	}

	// SnakeCaseFormat sets common keys as snake case
	SnakeCaseFormat = KeyFormat{
		ErrorKey:      "error",
		MessageKey:    "message",
		ResultKey:     "result",
		ResultSizeKey: "result_size",
		StatusKey:     "status",
	}
)
