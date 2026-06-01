package output

func SuccessMark() string { return colorize(successStyle, "✓") }
func ErrorMark() string   { return colorize(errorStyle, "✗") }
func WarnMark() string    { return colorize(warnStyle, "⚠") }
