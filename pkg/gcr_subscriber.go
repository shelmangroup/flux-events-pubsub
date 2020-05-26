package pkg

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/fluxcd/flux/pkg/remote/rpc"

	"cloud.google.com/go/pubsub"
	log "github.com/sirupsen/logrus"
)

type GCRMessage struct {
	Action string `json:"action"`
	Digest string `json:"digest"`
	Tag    string `json:"tag"`
}

func (g *GCRMessage) toJson() ([]byte, error) {
	data, err := json.Marshal(g)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (g *GCRMessage) fromJson(data []byte) error {
	err := json.Unmarshal(data, g)
	if err != nil {
		return err
	}
	return nil
}

func NewGCRSubscriber(projectID, topicID, subID string) (*GCRSubscriber, error) {
	ctx := context.Background()
	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		log.Errorf("pubsub.NewClient: %v", err)
		return nil, err
	}

	topic := client.Topic(topicID)
	topicExists, err := topic.Exists(ctx)
	if err != nil {
		return nil, err
	}
	if !topicExists {
		return nil, fmt.Errorf("topic %s doesn't exist", topicID)
	}

	sub := client.Subscription(subID)
	subExists, err := sub.Exists(ctx)
	if err != nil {
		return nil, err
	}

	if !subExists {
		sub, err = client.CreateSubscription(ctx, subID, pubsub.SubscriptionConfig{
			Topic:            topic,
			AckDeadline:      10 * time.Second,
			ExpirationPolicy: time.Duration(0),
		})
		if err != nil {
			return nil, err
		}
	}

	eventChan := make(chan []byte)
	return &GCRSubscriber{ctx: ctx, client: client, topic: topic, sub: sub, EventChan: eventChan}, nil
}

type GCRSubscriber struct {
	ctx       context.Context
	client    *pubsub.Client
	topic     *pubsub.Topic
	sub       *pubsub.Subscription
	fluxRPC   *rpc.RPCClientV11
	EventChan chan []byte
}

func (s *GCRSubscriber) Subscriber() error {
	cctx, _ := context.WithCancel(s.ctx)
	err := s.sub.Receive(cctx, func(ctx context.Context, msg *pubsub.Message) {
		message := GCRMessage{}
		err := message.fromJson(msg.Data)
		if err != nil {
			return
		}
		log.Infof("Got message: %#v\n", message)
		//v9.ImageChange
		//v9.ImageUpdate{}
	})
	if err != nil {
		return fmt.Errorf("Receive: %v", err)
	}
	return nil
}

func (s *GCRSubscriber) SendNotification(msg *GCRMessage) {

}
