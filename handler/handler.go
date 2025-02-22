package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/ShareFrame/update-profile-service/dynamodb"
	"github.com/ShareFrame/update-profile-service/models"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

var (
	hexColorRegex = regexp.MustCompile(`^#(?:[0-9a-fA-F]{3}){1,2}$`)
	urlRegex      = regexp.MustCompile(`^(https?://)?([a-zA-Z0-9.-]+)(:[0-9]+)?(/.*)?$`)
)

func validateProfile(profile models.UserProfile) error {
	logrus.WithField("received_nsid", profile.NSID).Info("Validating NSID")

	var validationErrors []string

	if profile.NSID != "social.shareframe.profile" {
		return errors.New("profile validation failed: invalid NSID: only 'social.shareframe.profile' is allowed")
	}

	validations := map[string]func() bool{
		"bio must be 256 characters or fewer": func() bool { return len(profile.Bio) > 256 },
		"invalid profilePicture URL":          func() bool { return profile.ProfilePicture != "" && !isValidURL(profile.ProfilePicture) },
		"invalid profileBanner URL":           func() bool { return profile.ProfileBanner != "" && !isValidURL(profile.ProfileBanner) },
		"theme must be 'light', 'dark', or 'custom'": func() bool {
			return profile.Theme != "" && profile.Theme != "light" && profile.Theme != "dark" && profile.Theme != "custom"
		},
		"primaryColor must be a valid hex code (e.g., #RRGGBB or #RGB)": func() bool {
			return profile.PrimaryColor != "" && !isValidHexColor(profile.PrimaryColor)
		},
		"secondaryColor must be a valid hex code (e.g., #RRGGBB or #RGB)": func() bool {
			return profile.SecondaryColor != "" && !isValidHexColor(profile.SecondaryColor)
		},
	}

	for errMsg, check := range validations {
		if check() {
			validationErrors = append(validationErrors, errMsg)
		}
	}

	if profile.UpdatedAt == "" {
		validationErrors = append(validationErrors, "updatedAt is required")
	} else if _, err := time.Parse(time.RFC3339, profile.UpdatedAt); err != nil {
		validationErrors = append(validationErrors, "invalid datetime format for updatedAt")
	}

	if len(validationErrors) > 0 {
		logrus.WithFields(logrus.Fields{
			"errors":  validationErrors,
			"profile": profile,
		}).Warn("Profile validation failed")

		return errors.New("profile validation failed: " + strings.Join(validationErrors, "; "))
	}

	logrus.Info("Profile validation passed")
	return nil
}

func isValidURL(url string) bool {
	return urlRegex.MatchString(url)
}

func isValidHexColor(color string) bool {
	return hexColorRegex.MatchString(color)
}

func HandleRequest(ctx context.Context, request models.RequestPayload) (models.UpdateProfileResponse, error) {
    logrus.WithField("did", request.DID).Info("Processing profile update request")

    dynamoClient, err := dynamodb.NewDynamoClient()
    if err != nil {
        logrus.WithError(err).Error("Failed to initialize DynamoDB client")
        return models.UpdateProfileResponse{Message: "Internal server error", Success: false}, err
    }

    profile := request.Profile

    if err := validateProfile(profile); err != nil {
        logrus.WithError(err).Warn("Profile validation failed")
        return models.UpdateProfileResponse{Message: "Profile validation failed", Success: false}, nil
    }

    logrus.WithField("profile", profile).Info("Validated profile successfully")

    _, err = updateProfileInATProtocol(request.DID, profile, request.AuthToken, false)
    if err != nil {
        logrus.WithError(err).Error("Failed to update profile in AT Protocol")
        return models.UpdateProfileResponse{Message: "Failed to update profile in AT Protocol", Success: false}, err
    }

    if err := dynamoClient.UpdateUserInDynamoDB(ctx, request.DID, profile); err != nil {
        logrus.WithError(err).Error("Failed to update profile in DynamoDB")
        return models.UpdateProfileResponse{Message: "Failed to update profile in database", Success: false}, err
    }

    logrus.WithField("did", request.DID).Info("Updated profile in DynamoDB successfully")
    logrus.Info("Profile update completed successfully")

    return models.UpdateProfileResponse{
        Message: "Profile updated successfully",
        Success: true,
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
