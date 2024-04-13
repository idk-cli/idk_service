package utils

import "strings"

func ContainsAnyIgnoreCase(str string, substrs []string) bool {
	// Convert the main string to lowercase
	strLower := strings.ToLower(str)

	// Iterate through each substring
	for _, substr := range substrs {
		// Convert the substring to lowercase
		substrLower := strings.ToLower(substr)

		// Check if the lowercase main string contains the lowercase substring
		if strings.Contains(strLower, substrLower) {
			// If any substring is contained, return true
			return true
		}
	}

	// If no substrings are contained, return false
	return false
}

func CleanGeminiJsonStr(geminiJsonStr string) string {
	cleanedPaLMResponse := strings.Replace(geminiJsonStr, "json", "", -1)
	cleanedPaLMResponse = strings.Replace(cleanedPaLMResponse, "```", "", -1)
	cleanedPaLMResponse = strings.Replace(cleanedPaLMResponse, "\n", "", -1)
	return cleanedPaLMResponse
}
