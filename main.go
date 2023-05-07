package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"

	dialogflow "cloud.google.com/go/dialogflow/apiv2"
	"cloud.google.com/go/dialogflow/apiv2/dialogflowpb"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/gin-gonic/gin"
)

type data struct {
	Text string `json:"text"`
}

func main() {
	r := gin.Default()
	r.POST("/chat", getIntentText)
	r.Run("localhost:8000")
}

func getIntentText(c *gin.Context) {
	var data data
	if err := c.BindJSON(&data); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	projectId := os.Getenv("PROJECT_ID")
	sessionId := os.Getenv("SESSION_ID")

	res, err := DetectIntentText(projectId, sessionId, data.Text, "en-us")
	if err != nil {
		fmt.Println(err.Error())
		c.JSON(http.StatusInternalServerError, "error occurred while processing")
		return
	}

	c.JSON(http.StatusOK, res)
}

func DetectIntentText(projectID, sessionID, text, languageCode string) (string, error) {
	fmt.Println(projectID, sessionID, text, languageCode)
	ctx := context.Background()

	sessionClient, err := dialogflow.NewSessionsClient(ctx)
	if err != nil {
		return "", err
	}
	defer sessionClient.Close()

	if projectID == "" || sessionID == "" {
		return "", errors.New(fmt.Sprintf("Received empty project (%s) or session (%s)", projectID, sessionID))
	}

	sessionPath := fmt.Sprintf("projects/%s/agent/sessions/%s", projectID, sessionID)
	textInput := dialogflowpb.TextInput{Text: text, LanguageCode: languageCode}
	queryTextInput := dialogflowpb.QueryInput_Text{Text: &textInput}
	queryInput := dialogflowpb.QueryInput{Input: &queryTextInput}
	request := dialogflowpb.DetectIntentRequest{Session: sessionPath, QueryInput: &queryInput}

	response, err := sessionClient.DetectIntent(ctx, &request)
	if err != nil {
		return "", err
	}

	queryResult := response.GetQueryResult()
	fulfillmentText := queryResult.GetFulfillmentText()
	go DescribeInstances()
	return fulfillmentText, nil
}

func DescribeInstances() {
	svc := ec2.New(session.New(&aws.Config{
		Region: aws.String("eu-north-1")}))

	input := &ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			{
				Name: aws.String("instance-state-name"),
				Values: []*string{
					aws.String("stopped"),
				},
			},
			{
				Name: aws.String("tag:Name"),
				Values: []*string{
					aws.String("test"),
				},
			},
		},
	}

	result, err := svc.DescribeInstances(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			default:
				fmt.Println(aerr.Error())
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			fmt.Println(err.Error())
		}
		return
	}

	for idx, res := range result.Reservations {
		fmt.Println("  > Reservation Id", *res.ReservationId, " Num Instances: ", len(res.Instances))
		for _, inst := range result.Reservations[idx].Instances {
			fmt.Println("    - Instance ID: ", *inst.InstanceId, " Tag name: ", *inst.Tags[0].Value)
		}
	}
}

func TerminateInstance(instanceIds []*string) {
	svc := ec2.New(session.New())
	input := &ec2.TerminateInstancesInput{
		InstanceIds: instanceIds,
	}

	result, err := svc.TerminateInstances(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			default:
				fmt.Println(aerr.Error())
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			fmt.Println(err.Error())
		}
		return
	}

	fmt.Println(result)
}
