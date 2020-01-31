package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"time"

	"cloud.google.com/go/pubsub"
	fluxevent "github.com/fluxcd/flux/pkg/event"
	"github.com/fluxcd/flux/pkg/http/websocket"
	"github.com/fluxcd/flux/pkg/remote/rpc"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	command       = kingpin.Command("server", "Start http server")
	listenAddress = command.Flag("listen-address", "HTTP address").Default(":8080").String()
	googleProject = command.Flag("google-project", "Google project").Required().String()
	pubsubTopic   = command.Flag("google-pubsub-topic", "Google pubsub topic").Required().String()
	labels        = command.Flag("labels", "Add additional labels to event, key=value").Short('l').StringMap()
)

func FullCommand() string {
	return command.FullCommand()
}

type Server struct {
	pubsubContext context.Context
	pubsubClient  *pubsub.Client
	rpcClient     *rpc.RPCClientV11
}

type ExtendedEvent struct {
	fluxevent.Event
	Labels map[string]string `json:"labels,omitempty"`
}

func NewServer() (*Server, error) {
	ctx := context.Background()
	c, err := pubsub.NewClient(ctx, *googleProject)
	if err != nil {
		log.Errorf("pubsub.NewClient: %v", err)
		return nil, err
	}

	return &Server{
		pubsubContext: ctx,
		pubsubClient:  c,
	}, nil
}

func (s *Server) Run() {
	r := mux.NewRouter()
	r.HandleFunc("/", s.websocketHandler)
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
			log.Error(err)
		}
	}()

	c := make(chan os.Signal, 1)
	// We'll accept graceful shutdowns when quit via SIGINT (Ctrl+C)
	// SIGKILL, SIGQUIT or SIGTERM (Ctrl+/) will not be caught.
	signal.Notify(c, os.Interrupt)

	// Block until we receive our signal.
	<-c

	// Create a deadline to wait for.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()
	srv.Shutdown(ctx)
	log.Infof("Shutting down")
}

func (s *Server) websocketHandler(w http.ResponseWriter, req *http.Request) {
	path := req.URL.Path
	c, err := websocket.Upgrade(w, req, nil)
	if err != nil {
		log.WithField("path", path).Errorf("Upgrade: %s", err)
		return
	}

	log.WithField("path", path).Infof("client connected")

	s.rpcClient = rpc.NewClientV11(c)
	ctx := context.Background()
	disconnect := make(chan struct{})
	defer close(disconnect)

	t := time.NewTicker(5 * time.Second)
	for {
		select {
		case <-t.C:
			err := s.rpcClient.Ping(ctx)
			if err != nil {
				log.WithField("path", path).Errorf("Ping: %s", err)
				return
			}
			v, err := s.rpcClient.Version(ctx)
			if err != nil {
				log.WithField("path", path).Errorf("Version: %s", err)
				return
			}
			log.WithField("path", path).Infof("Flux Version: %s", v)

		case <-disconnect:
			log.WithField("path", path).Infof("client disconnected")
			c.Close()
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

	t := s.pubsubClient.Topic(*pubsubTopic)
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
