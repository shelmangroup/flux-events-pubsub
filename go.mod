module github.com/shelmangroup/flux-events-pubsub

go 1.13

replace github.com/docker/distribution => github.com/2opremio/distribution v0.0.0-20190419185413-6c9727e5e5de

require (
	github.com/alecthomas/template v0.0.0-20190718012654-fb15b899a751 // indirect
	github.com/alecthomas/units v0.0.0-20190924025748-f65c72e2690d // indirect
	github.com/fluxcd/flux v1.17.1
	github.com/gorilla/mux v1.7.3
	github.com/gorilla/websocket v1.4.1
	github.com/sirupsen/logrus v1.4.2
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
)
