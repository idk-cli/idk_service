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

type RunHandler struct {
	geminiKey               string
	jwtKey                  []byte
	firestoreClient         *firestore.Client
	firestoreUserCollection string
	firestoreLogCollection  string
}

func NewRunHandler(geminiKey string, jwtKey []byte, firestoreClient *firestore.Client,
	firestoreUserCollection string, firestoreLogCollection string) *RunHandler {
	return &RunHandler{
		geminiKey:               geminiKey,
		jwtKey:                  jwtKey,
		firestoreClient:         firestoreClient,
		firestoreUserCollection: firestoreUserCollection,
		firestoreLogCollection:  firestoreLogCollection,
	}
}

func (h *RunHandler) HandleGetInitComamnds(c *gin.Context) {
	var req RunGetProjectInitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Request"})
		return
	}

	if len(req.Files) == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "files can not be empty"})
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

	response, err := h.getProjectTypeFromGemini(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error getting project type"})
		return
	}

	utils.IncreaseUsage(c, h.firestoreClient, h.firestoreUserCollection, tokenData.Email)

	utils.LogUserQuery(c, h.firestoreClient, h.firestoreLogCollection, tokenData.Email,
		"", req.OS, "", "", "GETPORJECTTYPE")

	c.JSON(http.StatusOK, &response)
}

func (h *RunHandler) getProjectTypeFromGemini(req RunGetProjectInitRequest) (*RunGetProjectInitResponse, error) {
	promptWithContext := fmt.Sprintf(`
        You help with running a project.

		This is project folder name: %s
        These are files of project: %s
    `, req.ProjectFolderName, strings.Join(req.Files, ","))
	if req.ReadmeFile != "" {
		promptWithContext = promptWithContext + fmt.Sprintf(`
			This is README of the project: %s
		`, req.ReadmeFile)
	}
	if req.MakeFile != "" {
		promptWithContext = promptWithContext + fmt.Sprintf(`
			This is MAKEFILE of the project: %s
		`, req.MakeFile)
	}

	promptWithContext = promptWithContext + fmt.Sprintf(`
		User has brew installed. But assume nothing else is installed.
		Provide all commands user needs to make sure I can run the project.
		(I am already in the project folder)
		Start from brew commands to install whatever is needed including langauge, tools etc
		then focus on commands to build the project depending on the language
		and then eventually last command should be to run the project
	   
	 	Your response should be in this format: 
		{\"projectType\": \"project type such as go, java etc project\", \"commands\": [{\"command\": \"command to execute\", \"description\":\"description of what command does\"}] } "
	`)

	result, err := clients.GenerateGemini(promptWithContext, h.geminiKey)
	if err != nil {
		return nil, err
	}

	println(result)

	cleanResult := utils.CleanGeminiJsonStr(result)

	println(cleanResult)

	var runGetProjectTypeResponse RunGetProjectInitResponse
	err = json.Unmarshal([]byte(cleanResult), &runGetProjectTypeResponse)
	if err != nil {
		return &RunGetProjectInitResponse{}, err
	}
	return &runGetProjectTypeResponse, nil
}

type RunGetProjectInitRequest struct {
	Files             []string `json:"files"`
	ReadmeFile        string   `json:"readme"`
	MakeFile          string   `json:"makefile"`
	OS                string   `json:"os"`
	ProjectFolderName string   `json:"projectFolderName"`
}

type RunGetProjectInitResponse struct {
	ProjectType string                     `json:"projectType"`
	Commands    []RunGetProjectInitCommand `json:"commands"`
}

type RunGetProjectInitCommand struct {
	Command     string `json:"command"`
	Description string `json:"description"`
}
