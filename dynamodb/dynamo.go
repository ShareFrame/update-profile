package dynamodb

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/ShareFrame/update-profile-service/models"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type DynamoDBAPI interface {
	UpdateItem(ctx context.Context, input *dynamodb.UpdateItemInput, opts ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error)
}

type DynamoDBService interface {
	UpdateUserInDynamoDB(ctx context.Context, userID string, profile models.UserProfile) error
}

var _ DynamoDBService = (*DynamoClient)(nil)

type DynamoClient struct {
	Client DynamoDBAPI
}

func NewDynamoClient() (*DynamoClient, error) {
	awsCfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS SDK config: %w", err)
	}

	dbClient := dynamodb.NewFromConfig(awsCfg)

	return &DynamoClient{
		Client: dbClient,
	}, nil
}

const tableName = "Users"

func (d *DynamoClient) UpdateUserInDynamoDB(ctx context.Context, userID string, profile models.UserProfile) error {
	if userID == "" {
		return fmt.Errorf("userID cannot be empty")
	}

	updateExpression, exprValues := buildUpdateExpression(profile)

	if len(exprValues) == 1 {
		return fmt.Errorf("no valid fields provided to update")
	}

	log.Printf("Updating UserId: %s, UpdateExpression: %s", userID, updateExpression)

	_, err := d.Client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:                 aws.String(tableName),
		Key:                       map[string]types.AttributeValue{"UserId": &types.AttributeValueMemberS{Value: userID}},
		UpdateExpression:          aws.String("SET " + updateExpression),
		ExpressionAttributeValues: exprValues,
		ReturnValues:              types.ReturnValueUpdatedNew,
	})

	if err != nil {
		log.Printf("DynamoDB UpdateItem error: %v", err)
		return fmt.Errorf("failed to update user in DynamoDB: %w", err)
	}

	return nil
}


func buildUpdateExpression(profile models.UserProfile) (string, map[string]types.AttributeValue) {
	fields := map[string]string{
		"DisplayName":    profile.DisplayName,
		"Bio":            profile.Bio,
		"ProfilePicture": profile.ProfilePicture,
		"ProfileBanner":  profile.ProfileBanner,
		"Theme":          profile.Theme,
		"PrimaryColor":   profile.PrimaryColor,
		"SecondaryColor": profile.SecondaryColor,
	}

	updateParts := []string{}
	exprValues := map[string]types.AttributeValue{}

	for field, value := range fields {
		if value != "" {
			updateParts = append(updateParts, field+" = :"+field)
			exprValues[":"+field] = &types.AttributeValueMemberS{Value: value}
		}
	}

	exprValues[":UpdatedAt"] = &types.AttributeValueMemberS{Value: time.Now().Format(time.RFC3339)}
	updateParts = append(updateParts, "UpdatedAt = :UpdatedAt")

	updateExpr := strings.Join(updateParts, ", ")

	return updateExpr, exprValues
}
