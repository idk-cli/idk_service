package handlers

import (
	"fmt"
	"idk_service/internal/clients"
	"idk_service/internal/utils"
	"net/http"

	"cloud.google.com/go/firestore"
	"github.com/gin-gonic/gin"
)

type DebugHandler struct {
	geminiKey               string
	jwtKey                  []byte
	firestoreClient         *firestore.Client
	firestoreUserCollection string
	firestoreLogCollection  string
}

func NewDebugHandler(geminiKey string, jwtKey []byte, firestoreClient *firestore.Client,
	firestoreUserCollection string, firestoreLogCollection string) *DebugHandler {
	return &DebugHandler{
		geminiKey:               geminiKey,
		jwtKey:                  jwtKey,
		firestoreClient:         firestoreClient,
		firestoreUserCollection: firestoreUserCollection,
		firestoreLogCollection:  firestoreLogCollection,
	}
}

func (h *DebugHandler) HandleDebugCommand(c *gin.Context) {
	var req DebugCommandRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Request"})
		return
	}

	if req.Command == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Command can not be empty"})
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

	response, err := h.getCommandDebugResultFromGemini(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error analyzing command error"})
		return
	}

	utils.IncreaseUsage(c, h.firestoreClient, h.firestoreUserCollection, tokenData.Email)

	utils.LogUserQuery(c, h.firestoreClient, h.firestoreLogCollection, tokenData.Email,
		req.Command, req.OS, "",
		response.Response, "COMMANDDEBUG")

	c.JSON(http.StatusOK, &response)
}

func (h *DebugHandler) getCommandDebugResultFromGemini(req DebugCommandRequest) (*DebugCommandResponse, error) {
	promptWithContext := fmt.Sprintf(`
        You help with finding terminal command errors.

        This is user's command: %s
		User is on OS: %s

		This is error from terminal: %s.

       Provide how user can fix that?"
    `, req.Command, req.OS, req.Error)

	result, err := clients.GenerateGemini(promptWithContext, h.geminiKey)
	if err != nil {
		return nil, err
	}

	return &DebugCommandResponse{
		Response: result,
	}, nil
}

type DebugCommandRequest struct {
	Command string `json:"command"`
	OS      string `json:"os"`
	Error   string `json:"error"`
}

type DebugCommandResponse struct {
	Response string `json:"response"`
}
