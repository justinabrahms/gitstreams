# OpenTelemetry Instrumentation

gitstreams includes optional OpenTelemetry instrumentation to help monitor and understand the sync operation performance.

## Features

The instrumentation provides visibility into:
- Overall sync operation duration
- Time to fetch followed users
- Per-user API call timings (starred repos, owned repos, events)
- Pagination details (pages fetched, items per page)

## Setup

### Prerequisites

You'll need an OpenTelemetry collector running. Here's a quick setup using Docker:

1. Create a `docker-compose.yml` file:

```yaml
version: '3'
services:
  otel-collector:
    image: otel/opentelemetry-collector:latest
    command: ["--config=/etc/otel-collector-config.yaml"]
    volumes:
      - ./otel-collector-config.yaml:/etc/otel-collector-config.yaml
    ports:
      - "4318:4318"   # OTLP HTTP receiver
      - "13133:13133" # Health check
      - "55679:55679" # zpages

  jaeger:
    image: jaegertracing/all-in-one:latest
    ports:
      - "16686:16686" # Jaeger UI
      - "14250:14250" # gRPC receiver
```

2. Create `otel-collector-config.yaml`:

```yaml
receivers:
  otlp:
    protocols:
      http:
        endpoint: 0.0.0.0:4318

exporters:
  debug:
    verbosity: detailed
  jaeger:
    endpoint: jaeger:14250
    tls:
      insecure: true

service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [debug, jaeger]
```

3. Start the collector:

```bash
docker-compose up -d
```

### Running gitstreams with OpenTelemetry

Set the OTEL environment variable to enable tracing:

```bash
export GITHUB_TOKEN=your_token_here
export OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4318
export OTEL_SERVICE_NAME=gitstreams  # Optional, defaults to "gitstreams"

./gitstreams
```

### Viewing Traces

Open Jaeger UI in your browser:

```
http://localhost:16686
```

Select the "gitstreams" service and explore the traces to see:
- Total sync operation time
- Per-user processing time
- API call breakdowns
- Pagination details

## Environment Variables

- `OTEL_EXPORTER_OTLP_ENDPOINT`: OTLP endpoint (required to enable tracing)
  - Format: `hostname:port` (e.g., `localhost:4318`)
- `OTEL_SERVICE_NAME`: Service name for traces (optional, defaults to "gitstreams")

## Spans

### Top-level spans

- `fetchActivity`: Overall sync operation
  - Attributes: `user_count` (number of users followed)

### Per-user spans

- `fetchUserActivity`: Processing a single user
  - Attributes: `user` (username)
- `getStarredRepos`: Fetching starred repositories
  - Attributes: `user` (username)
- `getOwnedRepos`: Fetching owned repositories
  - Attributes: `user` (username)
- `getRecentEvents`: Fetching recent events
  - Attributes: `user` (username)

### Pagination spans

- `github.getPaginated`: Overall pagination operation
  - Attributes: `path` (API path), `total_pages`, `total_results`
- `github.fetchPage`: Individual page fetch
  - Attributes: `path` (full path with params), `page` (page number), `results` (items in page)

## Example: Analyzing Performance

After running with OTEL enabled, you can:

1. Identify slow users: Look at `fetchUserActivity` spans to see which users take longest
2. Understand pagination overhead: Check `github.fetchPage` spans to see how many pages are fetched
3. API rate limiting: Monitor timing patterns to detect rate limit delays
4. Overall performance: Track `fetchActivity` duration over time

## Disabling Instrumentation

Simply omit the `OTEL_EXPORTER_OTLP_ENDPOINT` environment variable. The instrumentation code will use a no-op tracer with zero overhead.
