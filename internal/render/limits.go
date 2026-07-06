package render

const (
	DefaultMaxErrors = 5
	hideAllErrors    = -1
)

func errorLimit(totalErrors, maxErrors int) int {
	if totalErrors <= 0 || maxErrors == hideAllErrors {
		return 0
	}
	if maxErrors == 0 || maxErrors > totalErrors {
		return totalErrors
	}
	return maxErrors
}

func shouldPrintErrors(totalErrors, maxErrors int) bool {
	return errorLimit(totalErrors, maxErrors) > 0
}
