package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/shelmangroup/flux-events-pubsub/pkg"

	"cloud.google.com/go/pubsub"
	v9 "github.com/fluxcd/flux/pkg/api/v9"
	fluxevent "github.com/fluxcd/flux/pkg/event"
	"github.com/fluxcd/flux/pkg/http/websocket"
	"github.com/fluxcd/flux/pkg/remote/rpc"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	command                  = kingpin.Command("server", "Start http server")
	listenAddress            = command.Flag("listen-address", "HTTP address").Default(":8080").String()
	googleProject            = command.Flag("google-project", "Google project").Required().String()
	googleProjectGcrLocation = command.Flag("google-project-gcr", "Google project were gcr is located").Required().String()
	pubsubTopicEvents        = command.Flag("google-pubsub-topic", "Google pubsub topic for events").Required().String()
	pubsubTopicActions       = command.Flag("google-pubsub-topic-actions", "Google pubsub topic for actions").Required().String()
	pubsubSubscription       = command.Flag("google-pubsub-subscription", "Google pubsub subscription").Required().String()
	labels                   = command.Flag("labels", "Add additional labels to event, key=value").Short('l').StringMap()
)

func FullCommand() string {
	return command.FullCommand()
}

type Server struct {
	pubsubContext context.Context
	pubsubClient  *pubsub.Client
	broker        *Broker
	quit          chan struct{}
	gcrSubscriber *pkg.GCRSubscriber
}

type ExtendedEvent struct {
	fluxevent.Event
	Labels map[string]string `json:"labels,omitempty"`
}

type ActionEvent struct {
	Type string `json:"type"`
}

func NewServer() (*Server, error) {
	ctx := context.Background()
	c, err := pubsub.NewClient(ctx, *googleProject)
	if err != nil {
		log.Errorf("pubsub.NewClient: %v", err)
		return nil, err
	}

	gcrSubscriber, err := pkg.NewGCRSubscriber(ctx, *googleProjectGcrLocation, "gcr", "gcr")
	if err != nil {
		log.Errorf("pubsub.NewGCRSubscriber: %v", err)
		return nil, err
	}

	return &Server{
		pubsubContext: ctx,
		pubsubClient:  c,
		broker:        NewBroker(),
		quit:          make(chan struct{}),
		gcrSubscriber: gcrSubscriber,
	}, nil
}

func (s *Server) Run() {
	r := mux.NewRouter()
	r.HandleFunc("/v11/daemon", s.websocketHandler)
	r.HandleFunc("/v6/events", s.fluxEventV6Handler).Methods("POST")

	srv := &http.Server{
		Addr:         *listenAddress,
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
		Handler:      r,
	}

	go func() {
		log.Infof("Starting server on %s", *listenAddress)
		if err := srv.ListenAndServe(); err != nil {
			log.Errorf("http server error: %s", err)
		}
		close(s.quit)
	}()
	go func() {
		log.Infof("Listening for events on %s", *pubsubTopicActions)
		if err := s.subscriber(); err != nil {
			log.Errorf("subscriber error: %s", err)
		}
		close(s.quit)
	}()
	go func() {
		log.Infof("Listening for events on gcr topic")
		if err := s.gcrSubscriber.Subscriber(); err != nil {
			log.Errorf("gcr subscriber error: %s", err)
		}
		close(s.quit)
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	for {
		select {
		case <-c:
			log.Info("Interrupt")
			s.gracefulShutdown(srv)
			return
		case <-s.quit:
			log.Info("Quitting")
			s.gracefulShutdown(srv)
			return
		}
	}
}

func (s *Server) gracefulShutdown(srv *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()
	srv.Shutdown(ctx)
	s.pubsubClient.Close()
	return
}

func (s *Server) subscriber() error {
	ctx := context.Background()
	topic := s.pubsubClient.Topic(*pubsubTopicActions)
	topicExists, err := topic.Exists(ctx)
	if err != nil {
		return err
	}
	if !topicExists {
		return fmt.Errorf("topic %s doesn't exist", *pubsubTopicActions)
	}

	sub := s.pubsubClient.Subscription(*pubsubSubscription)
	subExists, err := sub.Exists(ctx)
	if err != nil {
		return err
	}

	if !subExists {
		sub, err = s.pubsubClient.CreateSubscription(ctx, *pubsubSubscription, pubsub.SubscriptionConfig{
			Topic:            topic,
			AckDeadline:      10 * time.Second,
			ExpirationPolicy: time.Duration(0),
		})
		if err != nil {
			return err
		}
	}

	err = sub.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
		s.broker.Notifier <- msg.Data
		msg.Ack()
	})
	if err != nil {
		return err
	}
	return nil
}

func (s *Server) websocketHandler(w http.ResponseWriter, req *http.Request) {
	path := req.URL.Path
	upgradedWebsocket, err := websocket.Upgrade(w, req, nil)
	if err != nil {
		log.WithField("path", path).Errorf("Upgrade: %s", err)
		return
	}
	defer upgradedWebsocket.Close()
	rpcClient := rpc.NewClientV11(upgradedWebsocket)
	ctx := req.Context()

	messageChan := make(chan []byte)
	s.broker.newClients <- messageChan
	defer func() {
		s.broker.closingClients <- messageChan
	}()

	version, err := rpcClient.Version(ctx)
	if err != nil {
		log.WithField("path", path).Errorf("Version: %s", err)
		return
	}
	log.WithField("path", path).Infof("client version: %s", version)

	for {
		select {
		case m := <-messageChan:
			log.WithField("path", path).Infof("Event: %s", string(m))
			err := rpcClient.NotifyChange(ctx, v9.Change{Kind: v9.GitChange})
			if err != nil {
				log.WithField("path", path).Errorf("NotifyChange: %s", err)
				return
			}
		case m := <-s.gcrSubscriber.EventChan:
			log.WithField("path", path).Infof("Event: %v", m)
			err := s.gcrSubscriber.SendNotification(m, rpcClient)
			if err != nil {
				log.WithField("path", path).Errorf("gcrSubscriber.SendNotification: %s", err)
				return
			}
		case <-ctx.Done():
			s.broker.closingClients <- messageChan
			return
		}
	}
}

func (s *Server) extendEvent(event fluxevent.Event, labels map[string]string) ([]byte, error) {
	extendedEvent := ExtendedEvent{
		Event:  event,
		Labels: labels,
	}
	e, err := json.Marshal(extendedEvent)
	if err != nil {
		return nil, err
	}
	return e, nil
}

func (s *Server) fluxEventV6Handler(w http.ResponseWriter, req *http.Request) {
	path := req.URL.Path
	event := fluxevent.Event{}

	eventStr, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.WithField("path", path).Error(err)
		http.Error(w, err.Error(), 400)
		return
	}

	err = json.NewDecoder(bytes.NewBuffer(eventStr)).Decode(&event)
	if err != nil {
		log.WithField("path", path).Error(err)
		http.Error(w, err.Error(), 400)
		return
	}

	if *labels != nil {
		eventStr, err = s.extendEvent(event, *labels)
		if err != nil {
			log.WithField("path", path).Error(err)
			http.Error(w, err.Error(), 400)
			return
		}
	}

	log.WithField("path", path).Debugf("Retrieved event: %s", eventStr)

	t := s.pubsubClient.Topic(*pubsubTopicEvents)
	result := t.Publish(s.pubsubContext, &pubsub.Message{Data: eventStr})
	id, err := result.Get(s.pubsubContext)
	if err != nil {
		log.WithField("path", path).Errorf("Get: %v", err)
		http.Error(w, err.Error(), 500)
		return
	}
	log.WithField("path", path).Infof("Published a event; msg ID: %v\n", id)

	w.WriteHeader(200)
	return
}
