package pkg

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/fluxcd/flux/pkg/image"

	v9 "github.com/fluxcd/flux/pkg/api/v9"

	"github.com/fluxcd/flux/pkg/remote/rpc"

	"cloud.google.com/go/pubsub"
	log "github.com/sirupsen/logrus"
)

/*
	{
		Action:"INSERT",
		Digest:"gcr.io/alex-dev-ne8701/testing@sha256:0b1e25fb646c62aff375bd5e6eb1b26a81abf2e9f5abb41f98acc88789c75434",
		Tag:"gcr.io/alex-dev-ne8701/testing:873245"
	}
*/
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

func NewGCRSubscriber(ctx context.Context, projectID, topicID, subID string) (*GCRSubscriber, error) {
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
	ctx, _ := context.WithCancel(s.ctx)
	err := s.sub.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
		message := &GCRMessage{}
		err := message.fromJson(msg.Data)
		if err != nil {
			return
		}
		log.Debugf("Subscriber got message: %#v\n", message)
		s.SendNotification(message)
	})
	if err != nil {
		return fmt.Errorf("Subscriber: %v", err)
	}
	return nil
}

func (s *GCRSubscriber) SendNotification(msg *GCRMessage) error {
	ref, err := image.ParseRef(msg.Tag)
	if err != nil {
		log.Errorf("SendNotification failed to parse message tag with err: %s", err)
		return err
	}
	change :=
		v9.Change{
			Kind: v9.ImageChange,
			Source: v9.ImageUpdate{
				Name: ref.Name,
			},
		}
	timeout := time.Second * 10
	ctx, _ := context.WithTimeout(s.ctx, timeout)
	err = s.fluxRPC.NotifyChange(ctx, change)
	if err != nil {
		log.Errorf("SendNotification failed to send flux rpc with err: %s", err)
		return err
	}

	return nil
}
