package handlers

import (
	"encoding/json"
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
	var needsPWD bool = false
	var needsFolderFileStrucutre bool = false

	var err error = nil
	if req.ExistingScript != "" {
		actionType = "SCRIPT"
	} else if req.ReadmeData != "" {
		actionType = "COMMANDFROMREADME"
	} else {
		actionResponse, err := h.getTypeFromGemini(req.Prompt)
		if actionResponse != nil {
			actionType = actionResponse.ActionType
			needsPWD = actionResponse.NeedsPWD
			needsFolderFileStrucutre = actionResponse.NeedsFolderFileStrucutre
		}

		if err != nil {
			return nil, err
		}
	}

	// pass requestPwd only if it's needed by ai
	requestPwd := ""
	if needsPWD {
		requestPwd = req.Pwd
	}
	// pass currentFolderFileStructure only if it's needed by ai
	currentFolderFileStructure := ""
	if needsFolderFileStrucutre {
		currentFolderFileStructure = req.CurrentFolderFileStructure
	}

	responsePrompt := ""
	switch actionType {
	case "COMMAND":
		responsePrompt, err = h.getCommandFromGemini(req.Prompt, req.OS, requestPwd, currentFolderFileStructure)
	case "COMMANDFROMREADME":
		responsePrompt, err = h.getCommandWithReadmeFromGemini(req.Prompt, req.OS, req.ReadmeData, requestPwd, currentFolderFileStructure)
	case "SCRIPT":
		responsePrompt, err = h.getScriptFromGemini(req.Prompt, req.ExistingScript, req.OS, requestPwd, currentFolderFileStructure)
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

func (h *PromptHandler) getTypeFromGemini(prompt string) (*GeminiActionResponse, error) {
	promptWithContext := fmt.Sprintf(`
    You help users using terminal experience."

    This is user's request: %s.

    Provide which type of request is it:
    COMMAND: If user is asking to create a single terminal command
    SCRIPT: If user is asking to perform multiple commands or expilicty mentioning script
    NONE: If it is something that is not a terminal request

    Your response should only be in this format:
	{
		"actionType": "string value of COMMAND, SCRIPT or NONE",
		"needsPWD": "boolean whether ai will need present working directory details to process user's request",
		"needsFolderFileStrucutre":  "boolean whether ai will need to know file structure of the project to process user's request"
	}
`, prompt)

	response, err := clients.GenerateGemini(promptWithContext, h.geminiKey)
	if err != nil {
		return nil, fmt.Errorf("Something went wrong. Please try again!")
	}

	cleanedJsonResponse := utils.GeminiGetCleanedJsonResponse(response)

	var actionResponse GeminiActionResponse

	// Unmarshal the JSON into the person struct
	err = json.Unmarshal([]byte(cleanedJsonResponse), &actionResponse)
	if err != nil {
		return nil, fmt.Errorf("Error parsing JSON: %s", err)
	}

	return &actionResponse, nil
}

func (h *PromptHandler) getCommandFromGemini(prompt string, os string, pwd string, currentFolderFileStructure string) (string, error) {
	promptWithContext := fmt.Sprintf(`
        You help with finding terminal commands for a user.

        This is user's request: %s.
		User is on OS: %s
    `, prompt, os)

	// Conditionally add the parts
	if pwd != "" {
		promptWithContext += fmt.Sprintf("User's current working directory is: %s\n", pwd)
	}
	if currentFolderFileStructure != "" {
		promptWithContext += fmt.Sprintf("User's current directory file stucture is: %s\n", currentFolderFileStructure)
	}

	// Add the final part of the prompt
	promptWithContext += "\nProvide relevant terminal command.\n\nYour response should be a terminal command only."

	command, err := clients.GenerateGemini(promptWithContext, h.geminiKey)
	if err != nil {
		return "", err
	}

	return command, nil
}

func (h *PromptHandler) getCommandWithReadmeFromGemini(prompt string, os string, readme string,
	pwd string, currentFolderFileStructure string) (string, error) {
	promptWithContext := fmt.Sprintf(`
        You help with finding terminal commands for a user.

        This is user's request: %s.
		User is on OS: %s

		This is readme of the script: %s
    `, prompt, os, readme)

	// Conditionally add the parts
	if pwd != "" {
		promptWithContext += fmt.Sprintf("User's current working directory is: %s\n", pwd)
	}
	if currentFolderFileStructure != "" {
		promptWithContext += fmt.Sprintf("User's current directory file stucture is: %s\n", currentFolderFileStructure)
	}

	// Add the final part of the prompt
	promptWithContext += "\nProvide relevant command.\n\nYour response should be a command only."

	command, err := clients.GenerateGemini(promptWithContext, h.geminiKey)
	if err != nil {
		return "", err
	}

	return command, nil
}

func (h *PromptHandler) getScriptFromGemini(prompt string, existingScript string, os string,
	pwd string, currentFolderFileStructure string) (string, error) {
	promptWithContext := fmt.Sprintf(`
		You help with make terminal script for a user.

        This is user's request: %s.
		User is on OS: %s
    `, prompt, os)

	// Conditionally add the parts
	if existingScript != "" {
		promptWithContext += fmt.Sprintf("This is current script: %s\n", existingScript)
	}
	if pwd != "" {
		promptWithContext += fmt.Sprintf("User's current working directory is: %s\n", pwd)
	}
	if currentFolderFileStructure != "" {
		promptWithContext += fmt.Sprintf("User's current directory file stucture is: %s\n", currentFolderFileStructure)
	}

	// Add the final part of the prompt
	if existingScript != "" {
		promptWithContext += "\nUpdate existing terminal script.\n\nYour response should only be script code."
	} else {
		promptWithContext += "\nMake terminal script.\n\nYour response should only be script code."
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
	Prompt                     string `json:"prompt"`
	OS                         string `json:"os"`
	ExistingScript             string `json:"existingScript"`
	ReadmeData                 string `json:"readmeData"`
	Pwd                        string `json:"pwd"`
	CurrentFolderFileStructure string `json:"currentFolderFileStructure"`
}

type PromptResponse struct {
	Response   string `json:"response"`
	ActionType string `json:"actionType"`
}

type GeminiActionResponse struct {
	ActionType               string `json:"actionType"`
	NeedsPWD                 bool   `json:"needsPWD"`
	NeedsFolderFileStrucutre bool   `json:"needsFolderFileStrucutre"`
}
