package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ShareFrame/update-profile-service/dynamodb"
	"github.com/ShareFrame/update-profile-service/models"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

func validateProfile(profile models.UserProfile) error {
	logrus.WithField("received_nsid", profile.NSID).Info("Validating NSID")

	var validationErrors []string

	if profile.NSID != "social.shareframe.profile" {
		validationErrors = append(validationErrors, "invalid NSID: only social.shareframe.profile is allowed")
	}
	if profile.DisplayName == "" {
		validationErrors = append(validationErrors, "displayName must be present")
	}
	if len(profile.Bio) > 256 {
		validationErrors = append(validationErrors, "bio must be 256 characters or fewer")
	}
	if profile.Theme != "" && profile.Theme != "light" && profile.Theme != "dark" && profile.Theme != "custom" {
		validationErrors = append(validationErrors, "theme must be light, dark, or custom")
	}
	if _, err := time.Parse(time.RFC3339, profile.UpdatedAt); err != nil {
		validationErrors = append(validationErrors, "invalid datetime format for updatedAt")
	}

	if len(validationErrors) > 0 {
		logrus.WithFields(logrus.Fields{
			"errors": validationErrors,
			"profile": profile,
		}).Warn("Profile validation failed")

		return errors.New("profile validation failed: " + strings.Join(validationErrors, "; "))
	}

	logrus.Info("Profile validation passed")
	return nil
}


func HandleRequest(ctx context.Context, request models.RequestPayload) (map[string]string, error) {
	logrus.WithField("raw_request", request).Info("Received request payload")
	logrus.WithField("repo", request.Repo).Info("Processing profile update request")

	dynamoClient, err := dynamodb.NewDynamoClient()
	if err != nil {
		logrus.WithError(err).Error("Failed to initialize DynamoDB client")
		return map[string]string{"error": "Internal server error"}, err
	}

	profile := request.Profile

	if err := validateProfile(profile); err != nil {
		logrus.WithError(err).Warn("Profile validation failed")
		return map[string]string{"error": err.Error()}, nil
	}

	logrus.WithField("profile", profile).Info("Validated profile successfully")

	response, err := updateProfileInATProtocol(request.Repo, profile, request.BearerToken, false)
	if err != nil {
		logrus.WithError(err).Error("Failed to update profile in AT Protocol")
		return map[string]string{"error": err.Error()}, nil
	}

	if err := dynamoClient.UpdateUserInDynamoDB(ctx, request.Repo, profile); err != nil {
		logrus.WithError(err).Error("Failed to update profile in DynamoDB")
		return map[string]string{"error": "Failed to update profile in database"}, err
	}

	logrus.WithField("repo", request.Repo).Info("Updated profile in DynamoDB successfully")
	logrus.Info("Profile update completed successfully")

	return map[string]string{
		"message":  "Profile updated successfully",
		"response": response,
	}, nil
}

func updateProfileInATProtocol(repo string, profile models.UserProfile, bearerToken string, validate bool) (string, error) {
	updateURL := "https://shareframe.social/xrpc/com.atproto.repo.putRecord"
	rkey := uuid.New().String()

	body, err := json.Marshal(map[string]interface{}{
		"repo":       repo,
		"collection": "social.shareframe.profile",
		"rkey":       rkey,
		"validate":   validate,
		"record":     profile,
	})
	if err != nil {
		logrus.WithError(err).Error("Failed to marshal profile update request")
		return "", err
	}

	req, err := http.NewRequest("POST", updateURL, bytes.NewBuffer(body))
	if err != nil {
		logrus.WithError(err).Error("Failed to create HTTP request")
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+bearerToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		logrus.WithError(err).Error("Failed to send profile update request to AT Protocol")
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logrus.WithFields(logrus.Fields{
			"status": resp.Status,
			"repo":   repo,
		}).Error("Profile update failed in AT Protocol")
		return "", fmt.Errorf("failed to update profile: %s", resp.Status)
	}

	logrus.Info("Profile successfully updated in AT Protocol")
	return "Profile successfully updated", nil
}
