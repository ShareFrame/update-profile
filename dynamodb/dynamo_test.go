package dynamodb

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/ShareFrame/update-profile-service/models"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockDynamoDBAPI struct {
	mock.Mock
}

func (m *MockDynamoDBAPI) UpdateItem(ctx context.Context, input *dynamodb.UpdateItemInput, opts ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
	args := m.Called(ctx, input)
	return args.Get(0).(*dynamodb.UpdateItemOutput), args.Error(1)
}

func TestUpdateUserInDynamoDB(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDynamoDB := new(MockDynamoDBAPI)
	client := &DynamoClient{Client: mockDynamoDB}

	testCases := []struct {
		name        string
		userID      string
		profile     models.UserProfile
		mockReturn  error
		expectError bool
		expectedSet []string
	}{
		{
			name:   "Successful update with all fields",
			userID: "user123",
			profile: models.UserProfile{
				DisplayName:    "John Doe",
				Bio:            "Test Bio",
				ProfilePicture: "picture.jpg",
				ProfileBanner:  "banner.jpg",
				Theme:          "dark",
				PrimaryColor:   "blue",
				SecondaryColor: "red",
			},
			mockReturn:  nil,
			expectError: false,
			expectedSet: []string{
				"DisplayName = :DisplayName",
				"Bio = :Bio",
				"ProfilePicture = :ProfilePicture",
				"ProfileBanner = :ProfileBanner",
				"Theme = :Theme",
				"PrimaryColor = :PrimaryColor",
				"SecondaryColor = :SecondaryColor",
				"UpdatedAt = :UpdatedAt",
			},
		},
		{
			name:   "Update with only some fields",
			userID: "user456",
			profile: models.UserProfile{
				DisplayName: "Jane Doe",
				Bio:         "Hello world",
			},
			mockReturn:  nil,
			expectError: false,
			expectedSet: []string{
				"DisplayName = :DisplayName",
				"Bio = :Bio",
				"UpdatedAt = :UpdatedAt",
			},
		},
		{
			name:   "Only UpdatedAt is updated when no fields are provided",
			userID: "user789",
			profile: models.UserProfile{},
			mockReturn:  nil,
			expectError: true, 
			expectedSet: []string{},
		},
		{
			name:   "DynamoDB returns an error",
			userID: "user-error",
			profile: models.UserProfile{
				DisplayName: "Error User",
			},
			mockReturn:  fmt.Errorf("DynamoDB update error"),
			expectError: true,
			expectedSet: []string{
				"DisplayName = :DisplayName",
				"UpdatedAt = :UpdatedAt",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockDynamoDB.ExpectedCalls = nil

			_, exprValues := buildUpdateExpression(tc.profile)

			if len(exprValues) == 1 {
				mockDynamoDB.On("UpdateItem", mock.Anything, mock.Anything).Return(nil, fmt.Errorf("no valid fields provided to update"))
			} else {
				mockDynamoDB.On("UpdateItem", mock.Anything, mock.MatchedBy(func(input *dynamodb.UpdateItemInput) bool {
					for _, expected := range tc.expectedSet {
						if !strings.Contains(*input.UpdateExpression, expected) {
							return false
						}
					}
					return true
				})).Return(&dynamodb.UpdateItemOutput{}, tc.mockReturn)
			}

			err := client.UpdateUserInDynamoDB(context.Background(), tc.userID, tc.profile)

			if tc.expectError {
				assert.Error(t, err, "Expected an error but got nil")
			} else {
				assert.NoError(t, err, "Unexpected error occurred")
			}

			mockDynamoDB.AssertExpectations(t)
		})
	}
}


func TestBuildUpdateExpression(t *testing.T) {
	testCases := []struct {
		name             string
		profile          models.UserProfile
		expectedSetParts []string
	}{
		{
			name: "All fields populated",
			profile: models.UserProfile{
				DisplayName:    "John Doe",
				Bio:            "Test Bio",
				ProfilePicture: "picture.jpg",
				ProfileBanner:  "banner.jpg",
				Theme:          "dark",
				PrimaryColor:   "blue",
				SecondaryColor: "red",
			},
			expectedSetParts: []string{
				"DisplayName = :DisplayName",
				"Bio = :Bio",
				"ProfilePicture = :ProfilePicture",
				"ProfileBanner = :ProfileBanner",
				"Theme = :Theme",
				"PrimaryColor = :PrimaryColor",
				"SecondaryColor = :SecondaryColor",
				"UpdatedAt = :UpdatedAt",
			},
		},
		{
			name: "Some fields populated",
			profile: models.UserProfile{
				DisplayName: "Jane Doe",
				Bio:         "Hello world",
			},
			expectedSetParts: []string{
				"DisplayName = :DisplayName",
				"Bio = :Bio",
				"UpdatedAt = :UpdatedAt",
			},
		},
		{
			name: "No fields populated (should only update UpdatedAt)",
			profile: models.UserProfile{},
			expectedSetParts: []string{
				"UpdatedAt = :UpdatedAt",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			updateExpression, exprValues := buildUpdateExpression(tc.profile)

			for _, part := range tc.expectedSetParts {
				assert.Contains(t, updateExpression, part, "Update expression missing expected field")
			}

			assert.Contains(t, exprValues, ":UpdatedAt", "UpdatedAt should always be in the expression values")
		})
	}
}

