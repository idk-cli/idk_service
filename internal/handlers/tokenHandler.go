package handlers

import (
	"encoding/json"
	"fmt"
	"idk_service/internal/utils"
	"io"
	"net/http"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/gin-gonic/gin"
)

type TokenHandler struct {
	jwtKey              []byte
	firestoreClient     *firestore.Client
	firestoreCollection string
}

func NewTokenHandler(jwtKey []byte, firestoreClient *firestore.Client, firestoreCollection string) *TokenHandler {
	return &TokenHandler{
		jwtKey:              jwtKey,
		firestoreClient:     firestoreClient,
		firestoreCollection: firestoreCollection,
	}
}

func (h *TokenHandler) CreateToken(c *gin.Context) {
	var req TokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Request"})
		return
	}

	googleUser, err := getGoogleUserInfo(req.AccessToken)

	// Verify the access token with Google
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid Token"})
		return
	}

	email := googleUser.Email

	// tokens are valid for 30 days
	expiryTime := time.Now().Add(30 * 24 * time.Hour)
	tokenString, err := utils.CreateTokenFromData(utils.TokenData{
		Email: email,
	}, expiryTime, h.jwtKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating a token"})
		return
	}

	user, err := utils.GetUser(c, h.firestoreClient, h.firestoreCollection, email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating a token"})
		return
	}

	if user == nil {
		// create new user if doesn't exist
		err = utils.SaveUser(c, h.firestoreClient, h.firestoreCollection, email, req.AccessToken, req.RefreshToken)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating a token"})
		return
	}

	// Respond with the JWT token and expiry timestamp
	c.JSON(http.StatusOK, TokenResponse{
		JWTToken: tokenString,
		Expiry:   expiryTime.Unix(),
	})
}

func getGoogleUserInfo(accessToken string) (*GoogleUserInfo, error) {
	reqURL := "https://www.googleapis.com/oauth2/v2/userinfo"
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", "Bearer "+accessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	var userInfo GoogleUserInfo
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(body, &userInfo); err != nil {
		return nil, err
	}

	return &userInfo, nil
}

type TokenRequest struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}

type TokenResponse struct {
	JWTToken string `json:"jwtToken"`
	Expiry   int64  `json:"expiry"`
}

type GoogleUserInfo struct {
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	// Include other fields as needed
}
