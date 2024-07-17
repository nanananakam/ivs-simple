package main

import (
	"context"
	"encoding/json"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dynamodbTypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/ivs"
	ivsTypes "github.com/aws/aws-sdk-go-v2/service/ivs/types"
	"github.com/aws/aws-sdk-go-v2/service/ivschat"
	ivschatTypes "github.com/aws/aws-sdk-go-v2/service/ivschat/types"
	"github.com/google/uuid"
	"log"
	"os"
)

type StartLiveStreamResponse struct {
	IngestEndpoint string `json:"ingest_endpoint"`
	StreamKey      string `json:"stream_key"`
}

type GetLiveStreamsResponse struct {
	Arns []string `json:"arns"`
}

type GetLiveStreamResponse struct {
	Arn         string `json:"arn"`
	PlaybackUrl string `json:"playback_url"`
	ChatToken   string `json:"chat_token"`
}

type ChannelInfo struct {
	Arn     string `dynamodbav:"arn"`
	ChatArn string `dynamodbav:"chat_arn"`
}

func startLiveStream(ctx context.Context) (events.LambdaFunctionURLResponse, error) {
	// Load the shared AWS configuration.
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion("ap-northeast-1"))
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}

	// Create an IVS service client
	ivsSvc := ivs.NewFromConfig(cfg)

	// Create a new IVS channel
	input := &ivs.CreateChannelInput{
		LatencyMode:    ivsTypes.ChannelLatencyModeLowLatency,
		Type:           ivsTypes.ChannelTypeBasicChannelType,
		InsecureIngest: true,
	}

	createChannelResult, err := ivsSvc.CreateChannel(ctx, input)
	if err != nil {
		log.Fatalf("failed to create IVS channel, %v", err)
	}

	ivschatSvc := ivschat.NewFromConfig(cfg)

	createRoomResult, err := ivschatSvc.CreateRoom(ctx, &ivschat.CreateRoomInput{})
	if err != nil {
		log.Fatalf("failed to create IVS chat room, %v", err)
	}

	channelInfo := ChannelInfo{
		Arn:     *createChannelResult.Channel.Arn,
		ChatArn: *createRoomResult.Arn,
	}
	channelInfoAv, err := attributevalue.MarshalMap(channelInfo)
	if err != nil {
		log.Fatalf("failed to marshal channelInfo, %v", err)
	}

	dynamodbSvc := dynamodb.NewFromConfig(cfg)

	if _, err := dynamodbSvc.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(os.Getenv("TABLE_NAME")),
		Item:      channelInfoAv,
	}); err != nil {
		log.Fatalf("failed to put item to DynamoDB, %v", err)
	}

	startLiveResponse := StartLiveStreamResponse{
		IngestEndpoint: *createChannelResult.Channel.IngestEndpoint,
		StreamKey:      *createChannelResult.StreamKey.Value,
	}

	startLiveResponseJson, err := json.Marshal(startLiveResponse)
	if err != nil {
		panic(err)
	}

	return events.LambdaFunctionURLResponse{
		StatusCode: 200,
		Body:       string(startLiveResponseJson),
	}, nil
}

func getLiveStreams(ctx context.Context) (events.LambdaFunctionURLResponse, error) {
	// Load the shared AWS configuration.
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(os.Getenv("REGION")))
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}

	// Create an IVS service client
	svc := ivs.NewFromConfig(cfg)

	// Prepare input for ListChannels API call
	input := &ivs.ListChannelsInput{}

	// Call ListChannels API
	var allChannels []ivsTypes.ChannelSummary
	paginator := ivs.NewListChannelsPaginator(svc, input)

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			log.Fatalf("failed to get IVS channels, %v", err)
		}

		allChannels = append(allChannels, page.Channels...)
	}

	arns := make([]string, 0)
	for _, channel := range allChannels {
		arns = append(arns, *channel.Arn)
	}

	getLiveStreamsResponse := GetLiveStreamsResponse{
		Arns: arns,
	}

	getLiveStreamsResponseJson, err := json.Marshal(getLiveStreamsResponse)
	if err != nil {
		log.Fatalf("failed to marshal getLiveStreamsResponse, %v", err)
	}

	return events.LambdaFunctionURLResponse{
		StatusCode: 200,
		Body:       string(getLiveStreamsResponseJson),
	}, nil

}
func getLiveStream(ctx context.Context, arn string) (events.LambdaFunctionURLResponse, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(os.Getenv("REGION")))
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}

	ivsSvc := ivs.NewFromConfig(cfg)

	input := &ivs.GetChannelInput{
		Arn: aws.String(arn),
	}

	getChannelOutput, err := ivsSvc.GetChannel(ctx, input)
	if err != nil {
		log.Fatalf("failed to get IVS channel, %v", err)
	}

	dynamodbSvc := dynamodb.NewFromConfig(cfg)

	getItemOutput, err := dynamodbSvc.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(os.Getenv("TABLE_NAME")),
		Key: map[string]dynamodbTypes.AttributeValue{
			"arn": &dynamodbTypes.AttributeValueMemberS{
				Value: arn,
			},
		},
	})
	if err != nil {
		log.Fatalf("failed to get item from DynamoDB, %v", err)
	}
	channelInfo := ChannelInfo{}
	if err := attributevalue.UnmarshalMap(getItemOutput.Item, &channelInfo); err != nil {
		log.Fatalf("failed to unmarshal channelInfo, %v", err)
	}

	ivschatSvc := ivschat.NewFromConfig(cfg)
	uuidV4, err := uuid.NewRandom()
	if err != nil {
		log.Fatalf("failed to generate uuid, %v", err)
	}

	createChatTokenInput := ivschat.CreateChatTokenInput{
		RoomIdentifier: aws.String(channelInfo.ChatArn),
		Capabilities:   []ivschatTypes.ChatTokenCapability{ivschatTypes.ChatTokenCapabilitySendMessage},
		UserId:         aws.String(uuidV4.String()),
	}

	createChatTokenOutput, err := ivschatSvc.CreateChatToken(ctx, &createChatTokenInput)
	if err != nil {
		log.Fatalf("failed to create chat token, %v", err)
	}

	getLiveStreamResponse := GetLiveStreamResponse{
		Arn:         *getChannelOutput.Channel.Arn,
		PlaybackUrl: *getChannelOutput.Channel.PlaybackUrl,
		ChatToken:   *createChatTokenOutput.Token,
	}

	getLiveStreamResponseJson, err := json.Marshal(getLiveStreamResponse)
	if err != nil {
		log.Fatalf("failed to marshal getLiveStreamResponse, %v", err)
	}

	return events.LambdaFunctionURLResponse{
		StatusCode: 200,
		Body:       string(getLiveStreamResponseJson),
	}, nil

}

func handler(ctx context.Context, request events.LambdaFunctionURLRequest) (events.LambdaFunctionURLResponse, error) {
	if request.RequestContext.HTTP.Method == "POST" {
		if request.RequestContext.HTTP.Path == "/start" {
			return startLiveStream(ctx)
		}
	}
	if request.RequestContext.HTTP.Method == "GET" {
		if request.RequestContext.HTTP.Path == "/streams" {
			return getLiveStreams(ctx)
		}
		if request.RequestContext.HTTP.Path == "/stream" {
			return getLiveStream(ctx, request.QueryStringParameters["arn"])
		}
	}
	return events.LambdaFunctionURLResponse{
		StatusCode: 404,
		Body:       "Not Found",
	}, nil
}
func main() {
	lambda.Start(handler)
}
