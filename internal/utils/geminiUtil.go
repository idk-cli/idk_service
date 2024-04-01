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
	trailingCommaPattern := regexp.MustCompile(`,(?=\s*[}\]])`)
	cleanedResponse = trailingCommaPattern.ReplaceAllString(cleanedResponse, "")

	return cleanedResponse
}
