package beater

import (
	"testing"
	"time"

	"github.com/e-travel/cloudwatchlogsbeat/config"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func Test_Stream_Next_WillGenerateCorrectNumberOfEvents(t *testing.T) {
	// stub the registry functions
	stubRegistryRead = func(*Stream) error { return nil }
	stubRegistryWrite = func(*Stream) error { return nil }

	group := &Group{
		Name:       "group",
		Prospector: &config.Prospector{},
	}

	// stub our expected events
	receivedEvents := []*cloudwatchlogs.OutputLogEvent{
		CreateOutputLogEvent("Event 1\n"),
		CreateOutputLogEvent("Event 2\n"),
		CreateOutputLogEvent("Event 3\n"),
	}

	events := []*Event{}

	// create the stream
	client := &MockCWLClient{}
	stream := NewStream("TestStream", group, client, &MockRegistry{}, make(chan bool))
	publisher := &MockPublisher{}
	stream.Publisher = publisher
	// stub the publisher
	publisher.On("Publish", mock.AnythingOfType("*beater.Event")).Return().Run(
		func(args mock.Arguments) {
			event := args.Get(0).(*Event)
			// add the event to the actual events
			events = append(events, event)
		})
	// stub the log events
	client.On("GetLogEvents", mock.AnythingOfType("*cloudwatchlogs.GetLogEventsInput")).Return(
		&cloudwatchlogs.GetLogEventsOutput{
			Events: receivedEvents,
		}, nil)
	// fire!
	stream.Next()
	// assert
	assert.Equal(t, len(receivedEvents), len(events))
}

// test stream cleanup (a message will be sent to the finished channel)
func Test_StreamShouldSendACleanupEvent_OnError(t *testing.T) {
	// stub the registry functions
	stubRegistryRead = func(*Stream) error { return nil }
	stubRegistryWrite = func(*Stream) error { return nil }

	client := &MockCWLClient{}
	beat := &Cloudwatchlogsbeat{
		AWSClient: client,
		Registry:  &MockRegistry{},
	}
	group := NewGroup("group", &config.Prospector{}, beat)

	// stub GetLogEvents to return the error
	client.On("GetLogEvents", mock.AnythingOfType("*cloudwatchlogs.GetLogEventsInput")).Return(
		nil, awserr.New(cloudwatchlogs.ErrCodeInvalidOperationException, "Error", nil))

	// create the finished channel
	finished := make(chan bool)
	stream := NewStream("TestStream", group, client, &MockRegistry{}, finished)
	// stub the log events
	client.On("GetLogEvents", mock.AnythingOfType("*cloudwatchlogs.GetLogEventsInput")).Return(
		nil, awserr.New(cloudwatchlogs.ErrCodeInvalidOperationException, "Error", nil))
	// fire!
	go stream.Monitor()
	// capture and assert the event
	assert.True(t, <-finished)
}

// test the stream sends an event on the finished channel on expiration
func Test_StreamShouldSendACleanupEvent_OnExpiring(t *testing.T) {
	t.Skip("pending")
}

func Test_StreamParams_HaveTheCorrectStartTime(t *testing.T) {
	horizon := time.Hour
	group := &Group{
		Name: "group",
		Prospector: &config.Prospector{
			StreamLastEventHorizon: horizon,
		},
	}

	// create the stream
	stream := NewStream("TestStream", group, nil, nil, nil)
	// create the events
	event1 := CreateOutputLogEventWithTimestamp("Event 1\n", TimeBeforeNowInMilliseconds(2*time.Hour))
	event2 := CreateOutputLogEventWithTimestamp("Event 2\n", TimeBeforeNowInMilliseconds(30*time.Minute))
	startTime := aws.Int64Value(stream.Params.StartTime)
	// assert
	assert.True(t, *event1.Timestamp < startTime)
	assert.True(t, *event2.Timestamp > startTime)
}
