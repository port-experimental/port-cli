package output

import (
	"fmt"
	"strings"
)

// ErrorContext provides additional context for errors.
type ErrorContext struct {
	Error      error
	Suggestion string
	HelpURL    string
	ErrorCode  string
}

// FormatError formats an error with context and suggestions.
func FormatError(err error) string {
	if err == nil {
		return ""
	}

	errMsg := err.Error()
	
	// Map common errors to helpful suggestions
	suggestion := getSuggestion(errMsg)
	errorCode := getErrorCode(errMsg)

	var parts []string
	parts = append(parts, Error(errMsg))
	
	if suggestion != "" {
		parts = append(parts, fmt.Sprintf("\n%s", Info("Suggestion: "+suggestion)))
	}
	
	if errorCode != "" {
		parts = append(parts, fmt.Sprintf("\n%s", Dim("Error code: "+errorCode)))
	}

	return strings.Join(parts, "")
}

// FormatErrorWithContext formats an error with explicit context.
func FormatErrorWithContext(ctx ErrorContext) string {
	if ctx.Error == nil {
		return ""
	}

	var parts []string
	parts = append(parts, Error(ctx.Error.Error()))

	if ctx.Suggestion != "" {
		parts = append(parts, fmt.Sprintf("\n%s", Info("Suggestion: "+ctx.Suggestion)))
	}

	if ctx.HelpURL != "" {
		parts = append(parts, fmt.Sprintf("\n%s", Info("Documentation: "+ctx.HelpURL)))
	}

	if ctx.ErrorCode != "" {
		parts = append(parts, fmt.Sprintf("\n%s", Dim("Error code: "+ctx.ErrorCode)))
	}

	return strings.Join(parts, "")
}

// getSuggestion returns a helpful suggestion based on the error message.
func getSuggestion(errMsg string) string {
	lowerMsg := strings.ToLower(errMsg)

	switch {
	case strings.Contains(lowerMsg, "configuration not found") || strings.Contains(lowerMsg, "config"):
		return "Run `port config --init` to create a configuration file"
	case strings.Contains(lowerMsg, "credentials") || strings.Contains(lowerMsg, "401") || strings.Contains(lowerMsg, "unauthorized"):
		return "Check your credentials. Run `port config --show` to view current configuration"
	case strings.Contains(lowerMsg, "file not found") || strings.Contains(lowerMsg, "no such file"):
		return "Check that the file path is correct and the file exists"
	case strings.Contains(lowerMsg, "organization") && strings.Contains(lowerMsg, "not found"):
		return "Verify the organization name is correct. Run `port config --show` to see configured organizations"
	case strings.Contains(lowerMsg, "403") || strings.Contains(lowerMsg, "forbidden"):
		return "Check that your credentials have the necessary permissions"
	case strings.Contains(lowerMsg, "404") || strings.Contains(lowerMsg, "not found"):
		return "The requested resource may not exist or you may not have access to it"
	case strings.Contains(lowerMsg, "timeout") || strings.Contains(lowerMsg, "connection"):
		return "Check your network connection and try again. If the problem persists, check the API URL"
	case strings.Contains(lowerMsg, "429") || strings.Contains(lowerMsg, "rate limit"):
		return "Rate limit exceeded. Please wait a moment and try again"
	case strings.Contains(lowerMsg, "invalid") && strings.Contains(lowerMsg, "resource"):
		return "Check the resource type is valid. Valid types: blueprints, entities, scorecards, actions, teams, users, automations, pages, integrations"
	case strings.Contains(lowerMsg, "validation"):
		return "Check the input data format matches the expected schema"
	default:
		return ""
	}
}

// getErrorCode extracts or generates an error code from the error message.
func getErrorCode(errMsg string) string {
	lowerMsg := strings.ToLower(errMsg)

	switch {
	case strings.Contains(lowerMsg, "configuration not found"):
		return "CONFIG_NOT_FOUND"
	case strings.Contains(lowerMsg, "credentials") || strings.Contains(lowerMsg, "401"):
		return "AUTH_FAILED"
	case strings.Contains(lowerMsg, "403"):
		return "PERMISSION_DENIED"
	case strings.Contains(lowerMsg, "404"):
		return "RESOURCE_NOT_FOUND"
	case strings.Contains(lowerMsg, "file not found"):
		return "FILE_NOT_FOUND"
	case strings.Contains(lowerMsg, "timeout"):
		return "TIMEOUT"
	case strings.Contains(lowerMsg, "429"):
		return "RATE_LIMIT"
	case strings.Contains(lowerMsg, "validation"):
		return "VALIDATION_ERROR"
	case strings.Contains(lowerMsg, "organization") && strings.Contains(lowerMsg, "not found"):
		return "ORG_NOT_FOUND"
	default:
		return ""
	}
}

// WrapError wraps an error with context.
func WrapError(err error, suggestion string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w\n%s", err, Info("Suggestion: "+suggestion))
}

// WrapErrorWithCode wraps an error with context and error code.
func WrapErrorWithCode(err error, suggestion string, code string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w\n%s\n%s", err, Info("Suggestion: "+suggestion), Dim("Error code: "+code))
}









