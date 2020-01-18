package server

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	command       = kingpin.Command("server", "Start http server")
	listenAddress = command.Flag("listen-address", "HTTP address").Default(":8080").String()

	upgrader = websocket.Upgrader{}
)

func FullCommand() string {
	return command.FullCommand()
}

type Server struct{}

func NewServer() *Server {
	return &Server{}
}

func (s *Server) Run() {
	r := mux.NewRouter()
	r.HandleFunc("/", s.websocketHandler)
	r.HandleFunc("/v6/events", s.fluxEventV6Handler)

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
	log.Debug("Websocket handler")
	c, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		log.Warnf("Upgrade: ", err)
		return
	}

	defer func() {
		log.Infof("client disconnected")
		c.Close()
	}()

	log.Info("client connected")
	for {
		mt, message, err := c.ReadMessage()
		if err != nil {
			log.Infof("read:", err)
			break
		}

		log.Infof("recv: %s", message)
		err = c.WriteMessage(mt, message)

		if err != nil {
			log.Infof("write:", err)
			break
		}
	}
	return
}

func (s *Server) fluxEventV6Handler(w http.ResponseWriter, req *http.Request) {
	log.Debug("Event handler")
	//TODO: Parse flux events and publish to a pubsub topic
}
