package main

import (
	"github.com/ShareFrame/update-profile-service/handler"
	"github.com/aws/aws-lambda-go/lambda"
)

func main() {
	lambda.Start(handler.HandleRequest)
}
