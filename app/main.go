package main

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ivs"
	"github.com/aws/aws-sdk-go-v2/service/ivs/types"
	"log"
	"os"
)

type StartLiveStreamResponse struct {
	IngestEndpoint string `json:"ingest_endpoint"`
	StreamKey      string `json:"stream_key"`
}

type GetLiveStreamsResponse struct {
	PlaybackURLs []string `json:"playback_urls"`
}

func startLiveStream(ctx context.Context) (events.LambdaFunctionURLResponse, error) {
	// Load the shared AWS configuration.
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion("ap-northeast-1"))
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}

	// Create an IVS service client
	svc := ivs.NewFromConfig(cfg)

	// Create a new IVS channel
	input := &ivs.CreateChannelInput{
		LatencyMode:    types.ChannelLatencyModeLowLatency,
		Type:           types.ChannelTypeBasicChannelType,
		InsecureIngest: true,
	}

	result, err := svc.CreateChannel(ctx, input)
	if err != nil {
		log.Fatalf("failed to create IVS channel, %v", err)
	}

	startLiveResponse := StartLiveStreamResponse{
		IngestEndpoint: *result.Channel.IngestEndpoint,
		StreamKey:      *result.StreamKey.Value,
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
	var allChannels []types.ChannelSummary
	paginator := ivs.NewListChannelsPaginator(svc, input)

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.TODO())
		if err != nil {
			log.Fatalf("failed to get IVS channels, %v", err)
		}

		allChannels = append(allChannels, page.Channels...)
	}

	playbackURLs := make([]string, 0)
	for _, channel := range allChannels {
		getStreamInput := &ivs.GetStreamInput{
			ChannelArn: channel.Arn,
		}

		stream, err := svc.GetStream(ctx, getStreamInput)
		if err != nil {
			var notBroadcastingErr *types.ChannelNotBroadcasting
			var notFoundErr *types.ResourceNotFoundException
			if errors.As(err, &notBroadcastingErr) || errors.As(err, &notFoundErr) {
				continue
			}
			log.Fatalf("failed to get IVS stream %s, %v", *(channel.Arn), err)
		}

		playbackURLs = append(playbackURLs, *stream.Stream.PlaybackUrl)
	}

	getLiveStreamsResponse := GetLiveStreamsResponse{
		PlaybackURLs: playbackURLs,
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
	}
	return events.LambdaFunctionURLResponse{
		StatusCode: 404,
		Body:       "Not Found",
	}, nil
}
func main() {
	lambda.Start(handler)
}
