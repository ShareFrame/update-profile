package dynamodb

import (
	"context"
	"testing"

	"github.com/ShareFrame/update-profile-service/models"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
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
	mockDynamoDB := new(MockDynamoDBAPI)
	client := &DynamoClient{Client: mockDynamoDB}

	testCases := []struct {
		name        string
		userID      string
		profile     models.UserProfile
		mockReturn  error
		expectError bool
	}{
		{
			name:   "Successful update",
			userID: "user123",
			profile: models.UserProfile{
				DisplayName: "John Doe",
				Bio:         "This is a test bio",
			},
			mockReturn:  nil,
			expectError: false,
		},
		{
			name:   "Update with empty fields (only UpdatedAt is set)",
			userID: "user456",
			profile: models.UserProfile{},
			mockReturn:  nil,
			expectError: false,
		},
		{
			name:        "Empty userID",
			userID:      "",
			profile:     models.UserProfile{DisplayName: "John Doe"},
			mockReturn:  nil,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.mockReturn != nil {
				mockDynamoDB.On("UpdateItem", mock.Anything, mock.Anything).Return((*dynamodb.UpdateItemOutput)(nil), tc.mockReturn)
			} else {
				mockDynamoDB.On("UpdateItem", mock.Anything, mock.Anything).Return(&dynamodb.UpdateItemOutput{}, nil)
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
		name          string
		profile       models.UserProfile
		expectedParts []string
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
			expectedParts: []string{
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
			expectedParts: []string{
				"DisplayName = :DisplayName",
				"Bio = :Bio",
				"UpdatedAt = :UpdatedAt",
			},
		},
		{
			name:    "No fields populated",
			profile: models.UserProfile{},
			expectedParts: []string{
				"UpdatedAt = :UpdatedAt",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			updateExpression, _ := buildUpdateExpression(tc.profile)

			if len(tc.expectedParts) == 1 && tc.expectedParts[0] == "UpdatedAt = :UpdatedAt" {
				assert.Equal(t, "UpdatedAt = :UpdatedAt", updateExpression, "Expected only UpdatedAt in update expression")
			} else {
				for _, part := range tc.expectedParts {
					assert.Contains(t, updateExpression, part, "Update expression missing expected field")
				}
			}
		})
	}
}

