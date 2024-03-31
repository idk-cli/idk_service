package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"idk_service/internal/utils"
	"io"
	"net/http"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type TokenHandler struct {
	jwtKey              []byte
	firestoreClient     *firestore.Client
	firestoreCollection string
	googleClientId      string
	googleClientSecret  string
}

func NewTokenHandler(jwtKey []byte, firestoreClient *firestore.Client,
	firestoreCollection string, googleClientId string, googleClientSecret string) *TokenHandler {
	return &TokenHandler{
		jwtKey:              jwtKey,
		firestoreClient:     firestoreClient,
		firestoreCollection: firestoreCollection,
		googleClientId:      googleClientId,
		googleClientSecret:  googleClientSecret,
	}
}

func (h *TokenHandler) CreateGoogleAuthCodeURL(c *gin.Context) {
	var req GoogleOAuthCodeUrlRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Request"})
		return
	}

	conf := &oauth2.Config{
		ClientID:     h.googleClientId,
		ClientSecret: h.googleClientSecret,
		Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email"},
		RedirectURL:  "http://localhost:7999/callback",
		Endpoint:     google.Endpoint,
	}

	url := conf.AuthCodeURL(req.State, oauth2.AccessTypeOffline)

	c.JSON(http.StatusOK, GoogleOAuthCodeUrlResponse{
		Url: url,
	})
}

func (h *TokenHandler) CreateGoogleAuthExchange(c *gin.Context) {
	var req GoogleOAuthCodeUrlRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Request"})
		return
	}

	conf := &oauth2.Config{
		ClientID:     h.googleClientId,
		ClientSecret: h.googleClientSecret,
		Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email"},
		RedirectURL:  "http://localhost:7999/callback",
		Endpoint:     google.Endpoint,
	}

	url := conf.AuthCodeURL(req.State, oauth2.AccessTypeOffline)

	c.JSON(http.StatusOK, GoogleOAuthCodeUrlResponse{
		Url: url,
	})
}

func (h *TokenHandler) CreateToken(c *gin.Context) {
	ctx := context.Background()

	var req TokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Request"})
		return
	}

	googleConf := &oauth2.Config{
		ClientID:     h.googleClientId,
		ClientSecret: h.googleClientSecret,
		Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email"},
		RedirectURL:  "http://localhost:7999/callback",
		Endpoint:     google.Endpoint,
	}

	googleAuth, err := googleConf.Exchange(ctx, req.GoogleAuthCode)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid Code"})
		return
	}

	googleUser, err := getGoogleUserInfo(googleAuth.AccessToken)

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
		err = utils.SaveUser(c, h.firestoreClient, h.firestoreCollection, email, googleAuth.AccessToken, googleAuth.RefreshToken)
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

type GoogleOAuthCodeUrlRequest struct {
	State string `json:"state"`
}

type GoogleOAuthCodeUrlResponse struct {
	Url string `json:"url"`
}

type TokenRequest struct {
	GoogleAuthCode string `json:"googleAuthCode"`
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
