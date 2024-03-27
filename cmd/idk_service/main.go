package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/gin-gonic/gin"

	"idk_service/internal/clients"
	"idk_service/internal/config"
	"idk_service/internal/handlers"
)

func main() {
	ctx := context.Background()

	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "default"
	}

	// Initialize configuration
	config.InitConfig(env)
	// Initialize Router
	router := gin.Default()
	// Initialize Firebase
	firebaseApp := clients.InitFirebase()
	// Obtain a Firestore client from the Firebase app
	firestoreClient, err := firebaseApp.Firestore(ctx)
	if err != nil {
		log.Fatalf("Failed to obtain Firestore client: %v", err)
	}
	defer firestoreClient.Close()

	jwtKeyStr := config.GetConfigValue("secrets.jwtKey").(string)
	jwtKey := []byte(jwtKeyStr)
	geminiKeyStr := config.GetConfigValue("secrets.geminiKey").(string)
	firebaseTokenCollectionStr := config.GetConfigValue("storage.firebaseUserCollection").(string)
	// set token handler
	tokenHandler := handlers.NewTokenHandler(jwtKey, firestoreClient, firebaseTokenCollectionStr)
	router.POST("/token", tokenHandler.CreateToken)
	// set prompt handler
	promptHandler := handlers.NewPromptHandler(geminiKeyStr, jwtKey, firestoreClient, firebaseTokenCollectionStr)
	router.POST("/prompt", promptHandler.HandlePrompt)

	// setPortAndRun starts router on a server port
	serverPort := config.GetConfigValue("server.port")
	router.Run(fmt.Sprintf(":%d", serverPort))
}
