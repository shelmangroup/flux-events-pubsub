FROM gcr.io/distroless/base-debian10
ADD flux-events-pubsub /
ENTRYPOINT ["/flux-events-pubsub", "server"]
