package main

import (
	"log"
	"os"

	"github.com/shelmangroup/flux-events-pubsub/server"
	"gopkg.in/alecthomas/kingpin.v2"
)

func main() {
	kingpin.HelpFlag.Short('h')
	kingpin.CommandLine.DefaultEnvars()
	kingpin.Parse()

	log.SetOutput(os.Stderr)

	var err error

	switch kingpin.Parse() {
	case server.FullCommand():
		s := server.NewServer()
		s.Run()
	}
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	return
}
