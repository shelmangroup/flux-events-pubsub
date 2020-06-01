![CI](https://github.com/shelmangroup/flux-events-pubsub/workflows/CI/badge.svg?branch=master)
# Flux-events-pubsub

Let your [FluxCD](https://fluxcd.io) daemon connect to flux-events-pubsub as
upstream controlplane. Flux-events-pubsub will subscribe to a Google pubsub topic
and notify the FluxCD daemons upon changes which will trigger a fluxCD Sync.

FluxCD events (Commit, Sync and Release events) will also be published to pubsub topic
for later consumtion of different services.

FluxCD events for Google container regetry updates. (no updates are sent to other topics
for this changes)

## Example use cases that consume pubsub events.
- Github WebHooks
- Chat bots
- Trigger test suites
- etc.

## Example PubSub conf for gcr events
```bash
gcloud pubsub topics create gcr
```

## Example deployment


Kubernetes manifest example
```yaml
---
apiVersion: v1
kind: Service
metadata:
  name: flux-events
  namespace: flux
  labels:
    app: flux-events
spec:
  type: ClusterIP
  selector:
    app: flux-events
  ports:
  - name: http
    port: 8080
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: flux-events
  namespace: flux
  labels:
    app: flux-events
spec:
  replicas: 1
  selector:
    matchLabels:
      app: flux-events
  template:
    metadata:
      labels:
        app: flux-events
    spec:
      containers:
      - image: quay.io/shelman/flux-events-pubsub:latest
        name: flux-events
        args:
          - --log-level=debug
          - --log-json
          # Optionally add labels that will be append to the FluxCD json payload event.
          - -lmy-custom-label1=foo
          - -lmy-custom-label2=bar
        env:
          - name: FLUX_EVENTS_PUBSUB_GOOGLE_PROJECT_GCR
            value: "my gcr project"
          - name: GOOGLE_APPLICATION_CREDENTIALS
            value: "/secrets/pubsub-credentials"
          - name: FLUX_EVENTS_PUBSUB_GOOGLE_PROJECT
            value: "my-google-project"
          - name: FLUX_EVENTS_PUBSUB_GOOGLE_PUBSUB_TOPIC
            value: "flux-events"
            # Flux-events-pubsub will subscribe to this topic to trigger FluxCD Sync Actions.
          - name: FLUX_EVENTS_PUBSUB_GOOGLE_PUBSUB_TOPIC_ACTIONS
            value: "flux-actions"
          - name: FLUX_EVENTS_PUBSUB_GOOGLE_PUBSUB_SUBSCRIPTION
            valueFrom:
              fieldRef:
                fieldPath: metadata.name
        resources:
          requests:
            cpu: 50m
            memory: 64Mi
          limits:
            cpu: 200m
            memory: 128Mi
        volumeMounts:
          - mountPath: /secrets
            name: gcp-credentials
      volumes:
        - name: gcp-credentials
          secret:
            secretName: gcp-credentials
            defaultMode: 0600
```

### Connect FluxCD with an upstream
Add `--connect=ws://flux-events:8080` argument to fluxcd deployment manifest.
