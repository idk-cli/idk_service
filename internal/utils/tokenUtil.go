package utils

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt"
)

func GetDataFromToken(tokenString string, jwtKey []byte) (*TokenData, error) {
	// Parse the token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Validate the alg is what you expect:
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		return jwtKey, nil
	})

	if err != nil {
		return nil, err
	}

	// Extract and print the claims
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return &TokenData{
			Email: claims["email"].(string),
		}, nil
	} else {
		return nil, fmt.Errorf("Invalid token or claims")
	}
}

func CreateTokenFromData(tokenData TokenData, expiry time.Time, jwtKey []byte) (string, error) {
	// Create the JWT token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"exp":   expiry.Unix(),
		"email": tokenData.Email,
	})

	// Sign the token with your secret key
	tokenString, err := token.SignedString(jwtKey)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

type TokenData struct {
	Email string
}
