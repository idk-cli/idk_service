package clients

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	geminiAIBaseURL = "https://generativelanguage.googleapis.com/v1beta/models/gemini-pro:generateContent"
)

func GenerateGemini(prompt string, geminiAIKey string) (string, error) {
	// Constructing the request body directly
	requestBodyMap := map[string]interface{}{
		"contents": []interface{}{
			map[string]interface{}{
				"parts": []interface{}{
					map[string]string{
						"text": prompt,
					},
				},
			},
		},
	}

	requestBodyBytes, err := json.Marshal(requestBodyMap)
	if err != nil {
		return "", err
	}

	requestUrl := fmt.Sprintf("%s?key=%s", geminiAIBaseURL, geminiAIKey)
	response, err := http.Post(requestUrl, "application/json", bytes.NewBuffer(requestBodyBytes))
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("server returned non-OK status: %d", response.StatusCode)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}

	var responseData map[string]interface{}
	err = json.Unmarshal(body, &responseData)
	if err != nil {
		return "", err
	}

	// Navigating through the nested JSON response to extract the desired value
	candidates, ok := responseData["candidates"].([]interface{})
	if !ok || len(candidates) == 0 {
		return "", fmt.Errorf("no candidates found")
	}

	firstCandidate, ok := candidates[0].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("first candidate is not a valid object")
	}

	content, ok := firstCandidate["content"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("content is not a valid object")
	}

	parts, ok := content["parts"].([]interface{})
	if !ok || len(parts) == 0 {
		return "", fmt.Errorf("no parts found in content")
	}

	firstPart, ok := parts[0].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("first part is not a valid object")
	}

	text, ok := firstPart["text"].(string)
	if !ok {
		return "", fmt.Errorf("text is not a valid string")
	}

	return text, nil
}
