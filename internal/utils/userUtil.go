package utils

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	DAILY_LIMIT = 50
)

func GetUser(ctx context.Context, client *firestore.Client, collection string, email string) (*IDKUser, error) {
	docRef := client.Collection(collection).Doc(email)
	docSnapshot, err := docRef.Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, nil
		}

		return nil, err
	}

	if !docSnapshot.Exists() {
		return nil, nil
	}

	docData := docSnapshot.Data()

	return &IDKUser{
		Created:          docData["created"].(time.Time),
		UsageRefreshTime: docData["usageRefreshTime"].(time.Time),
		Usage:            docData["usage"].(int64),
	}, nil
}

func SaveUser(ctx context.Context, client *firestore.Client, collection string, email string, accessToken string, refreshToken string) error {
	// Save the key/value pair in Firestore
	_, err := client.Collection(collection).Doc(email).Set(ctx, map[string]interface{}{
		"created":          time.Now(),
		"usageRefreshTime": time.Now(),
		"usage":            0,
	}, firestore.MergeAll)

	if err != nil {
		return err
	}

	return nil
}

func ValidateUserLimit(ctx context.Context, client *firestore.Client, collection string, email string) error {
	user, err := GetUser(ctx, client, collection, email)
	if err != nil {
		return err
	}

	if user == nil {
		return fmt.Errorf("user doesn't exist")
	}

	if time.Since(user.UsageRefreshTime) > 24*time.Hour {
		_, err := client.Collection(collection).Doc(email).Set(ctx, map[string]interface{}{
			"usageRefreshTime": time.Now(),
			"usage":            0,
		}, firestore.MergeAll)

		if err != nil {
			return err
		}

		return nil
	}

	if user.Usage >= DAILY_LIMIT {
		return fmt.Errorf("Daily Quota limit reached")
	}

	return nil
}

func IncreaseUsage(ctx context.Context, client *firestore.Client, collection string, email string) error {
	_, err := client.Collection(collection).Doc(email).Update(ctx, []firestore.Update{
		{
			Path:  "usage",
			Value: firestore.Increment(1),
		},
	})

	return err
}

type IDKUser struct {
	Created          time.Time
	Usage            int64
	UsageRefreshTime time.Time
}
