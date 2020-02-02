[![Docker Repository on Quay](https://quay.io/repository/shelman/flux-events-pubsub/status "Docker Repository on Quay")](https://quay.io/repository/shelman/flux-events-pubsub)

# Flux-events-pubsub

Publish and subscribe fluxcd events on pubsub

```
usage: flux-events-pubsub [<flags>] <command> [<args> ...]

Flags:
  -h, --help            Show context-sensitive help (also try --help-long and --help-man).
      --log-json        Use structured logging in JSON format
      --log-fluentd     Use structured logging in GKE Fluentd format
      --log-level=info  The level of logging

Commands:
  help [<command>...]
    Show help.


  server --google-project=GOOGLE-PROJECT --google-pubsub-topic=GOOGLE-PUBSUB-TOPIC --google-pubsub-topic-actions=GOOGLE-PUBSUB-TOPIC-ACTIONS --google-pubsub-subscription=GOOGLE-PUBSUB-SUBSCRIPTION [<flags>]
    Start http server

        --listen-address=":8080"  HTTP address
        --google-project=GOOGLE-PROJECT
                                  Google project
        --google-pubsub-topic=GOOGLE-PUBSUB-TOPIC
                                  Google pubsub topic for events
        --google-pubsub-topic-actions=GOOGLE-PUBSUB-TOPIC-ACTIONS
                                  Google pubsub topic for actions
        --google-pubsub-subscription=GOOGLE-PUBSUB-SUBSCRIPTION
                                  Google pubsub subscription
    -l, --labels=LABELS ...       Add additional labels to event, key=value```
