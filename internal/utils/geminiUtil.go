package utils

import (
	"regexp"
	"strings"
)

func GeminiGetCleanedJsonResponse(geminiResponse string) string {
	cleanedResponse := strings.Replace(geminiResponse, "json", "", 1)
	cleanedResponse = strings.ReplaceAll(cleanedResponse, "```", "")
	cleanedResponse = strings.ReplaceAll(cleanedResponse, "\n", "")

	// Also clean trailing commas
	// Adjusted pattern to match a comma followed by any number of whitespace characters
	// and then a closing bracket or brace, without using a lookahead.
	trailingCommaPattern := regexp.MustCompile(`,\s*([}\]])`)
	cleanedResponse = trailingCommaPattern.ReplaceAllString(cleanedResponse, "$1")

	return cleanedResponse
}
