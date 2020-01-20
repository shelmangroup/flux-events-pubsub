FROM golang:1.13-buster as builder
WORKDIR /go/src
ADD . /go/src
RUN go get -d -v ./...
RUN go build -o /go/bin/flux-events-pubsub

FROM gcr.io/distroless/base-debian10
COPY --from=builder /go/bin/flux-events-pubsub /
ENTRYPOINT ["/flux-events-pubsub", "server"]
