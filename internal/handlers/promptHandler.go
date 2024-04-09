package handlers

import (
	"fmt"
	"idk_service/internal/clients"
	"idk_service/internal/utils"
	"net/http"
	"strings"

	"cloud.google.com/go/firestore"
	"github.com/gin-gonic/gin"
)

type PromptHandler struct {
	geminiKey               string
	jwtKey                  []byte
	firestoreClient         *firestore.Client
	firestoreUserCollection string
	firestoreLogCollection  string
}

func NewPromptHandler(geminiKey string, jwtKey []byte, firestoreClient *firestore.Client,
	firestoreUserCollection string, firestoreLogCollection string) *PromptHandler {
	return &PromptHandler{
		geminiKey:               geminiKey,
		jwtKey:                  jwtKey,
		firestoreClient:         firestoreClient,
		firestoreUserCollection: firestoreUserCollection,
		firestoreLogCollection:  firestoreLogCollection,
	}
}

func (h *PromptHandler) HandlePrompt(c *gin.Context) {
	var req PromptRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Request"})
		return
	}

	if req.Prompt == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Prompt can not be empty"})
		return
	}

	token := c.GetHeader("Authorization")
	tokenData, err := utils.GetDataFromToken(token, h.jwtKey)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	err = utils.ValidateUserLimit(c, h.firestoreClient, h.firestoreUserCollection, tokenData.Email)
	if err != nil {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": err.Error()})
		return
	}

	response, err := h.processPrompt(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error processing prompt"})
		return
	}

	utils.IncreaseUsage(c, h.firestoreClient, h.firestoreUserCollection, tokenData.Email)

	utils.LogUserQuery(c, h.firestoreClient, h.firestoreLogCollection, tokenData.Email,
		req.Prompt, req.OS, req.ExistingScript,
		response.Response, response.ActionType)

	c.JSON(http.StatusOK, &response)
}

func (h *PromptHandler) processPrompt(req PromptRequest) (*PromptResponse, error) {
	actionType := ""
	var err error = nil
	if req.ExistingScript != "" {
		actionType = "SCRIPT"
	} else if req.ReadmeData != "" {
		actionType = "COMMANDFROMREADME"
	} else if utils.ContainsAnyIgnoreCase(req.Prompt, []string{"go to", "cd"}) {
		actionType = "CD"
	} else {
		actionType, err = h.getTypeFromGemini(req.Prompt)
	}

	if err != nil {
		return nil, err
	}

	responsePrompt := ""
	switch actionType {
	case "COMMAND":
		responsePrompt, err = h.getCommandFromGemini(req.Prompt, req.OS)
	case "CD":
		responsePrompt, err = h.getCDFolderFromGemini(req.Prompt)
	case "COMMANDFROMREADME":
		responsePrompt, err = h.getCommandWithReadmeFromGemini(req.Prompt, req.OS, req.ReadmeData)
	case "SCRIPT":
		responsePrompt, err = h.getScriptFromGemini(req.Prompt, req.ExistingScript, req.OS)
	default:
		responsePrompt = "I also don't know"
	}

	if err != nil {
		return nil, err
	}

	return &PromptResponse{
		Response:   responsePrompt,
		ActionType: actionType,
	}, nil
}

func (h *PromptHandler) getTypeFromGemini(prompt string) (string, error) {
	promptWithContext := fmt.Sprintf(`
    You help users using terminal experience."

    This is user's request: %s.

    Provide which type of request is it:
    COMMAND: If user is asking to create a single terminal command
    SCRIPT: If user is asking to perform multiple commands or expilicty mentioning script
    NONE: If it is something that is not a terminal request

    Your response should only be type COMMAND, SCRIPT or NONE
`, prompt)

	actionType, err := clients.GenerateGemini(promptWithContext, h.geminiKey)
	if err != nil {
		return "", fmt.Errorf("Something went wrong. Please try again!")
	}

	return actionType, nil
}

func (h *PromptHandler) getCommandFromGemini(prompt string, os string) (string, error) {
	promptWithContext := fmt.Sprintf(`
        You help with finding terminal commands for a user.

        This is user's request: %s.
		User is on OS: %s

        Provide relevant terminal command.

        Your response should be a terminal command only.
    `, prompt, os)

	command, err := clients.GenerateGemini(promptWithContext, h.geminiKey)
	if err != nil {
		return "", err
	}

	return command, nil
}

func (h *PromptHandler) getCDFolderFromGemini(prompt string) (string, error) {
	promptWithContext := fmt.Sprintf(`
        Provide folder name where user wants to go.

        This is user's request: %s.

        Your response should only be folder name.
    `, prompt)

	command, err := clients.GenerateGemini(promptWithContext, h.geminiKey)
	if err != nil {
		return "", err
	}

	return command, nil
}

func (h *PromptHandler) getCommandWithReadmeFromGemini(prompt string, os string, readme string) (string, error) {
	promptWithContext := fmt.Sprintf(`
        You help with finding terminal commands for a user.

        This is user's request: %s.
		User is on OS: %s

		This is readme of the script: %s

        Provide relevant command.

        Your response should be a command only.
    `, prompt, os, readme)

	command, err := clients.GenerateGemini(promptWithContext, h.geminiKey)
	if err != nil {
		return "", err
	}

	return command, nil
}

func (h *PromptHandler) getScriptFromGemini(prompt string, existingScript string, os string) (string, error) {
	promptWithContext := ""
	if existingScript != "" {
		promptWithContext = fmt.Sprintf(`
        You help with make terminal script for a user.

        This is user's request: %s.

        This is current script: %s

		User is on OS: %s

        Update existing terminal script.

        Your response should only be script code.`, prompt, existingScript, os)
	} else {
		promptWithContext = fmt.Sprintf(`
        You help with make terminal script for a user.

        This is user's request: %s.

		User is on OS: %s

        Make terminal script.

        Your response should only be script code.`, prompt, os)
	}

	script, err := clients.GenerateGemini(promptWithContext, h.geminiKey)

	if err != nil {
		return "", fmt.Errorf("Something went wrong. Please try again!")
	}

	// Remove "```sh"
	cleanedScript := strings.Replace(script, "```sh", "", -1)
	// Remove "```"
	cleanedScript = strings.Replace(cleanedScript, "```", "", -1)

	return cleanedScript, nil
}

type PromptRequest struct {
	Prompt         string `json:"prompt"`
	OS             string `json:"os"`
	ExistingScript string `json:"existingScript"`
	ReadmeData     string `json:"readmeData"`
	Pwd            string `json:"pwd"`
}

type PromptResponse struct {
	Response   string `json:"response"`
	ActionType string `json:"actionType"`
}
