# Site Audit

## Background
Demonstrates a crawler application written in Go to build a graph of available links on a web page. The resulting output will be in the `out` folder in the root as a Graph Viz dot file.

This crawler leverages concurrency and [data structures](https://www.github.com/salsgithub/godst) to ensure no re-visits to visited links as well as not exploring external links from the host.

Improvements that could be made;

- **Handling redirects gracefully** - Configure the `HTTPFetcher` with a maximum redirect depth

- **Handling rate limits/too many requests** - This can be done in many ways but here are two; 
    - **Token bucket**: Use a token bucket strategy to throttle worker routines such that workers must obtain a token before making requests. A separate process adds tokens to the bucket at a fixed rate (e.g every two seconds).
    - **Retry**: When a `429` is received, don't discard the task. Calculate an exponential backoff duration. Re-queue the task after the duration for re-processing. Keep a count of maximum retries before discarding or storing in a DLQ if still unsuccessful.

- **Cache forbidden links** - The worker will start by respecting `robots.txt` when configured. Forbidden links could be cached as they could exist as anchor tags on more than one page.

## Prerequisites

- [Go >= 1.24](https://go.dev/doc/install)
- [GNU Make](https://www.gnu.org/software/make/)
- [Docker](https://docs.docker.com/engine/install/) (Optional)

## Getting started

For ease of use, a `Makefile` is in the root of the project to perform different tasks. To find out which tasks can be executed, run:

```sh
make help
```

## Configuration

You can configure the auditor using the following `.env` file placed in the root of the project when run locally

| Environment Variable | Default Value | Description |
| -------------------- | ------------- | ----------- |
| `AUDIT_LOG_LEVEL`    | `info`         | The logging level
| `AUDIT_START_URL`    | `https://google.com/` | The start url to crawl from |
| `AUDIT_AGENT`        | `agent` | The user-agent name|
| `AUDIT_VALID_SCHEMES`| `https`         | The schemes to allow when fetching |
| `AUDIT_RESPECT_ROBOTS`| `TRUE` | Respects the robots.txt file (this will be the first request made when set to true) |
| `AUDIT_MAX_WORKERS`  | `100` | The maximum number of workers to use |
| `AUDIT_MAX_DEPTH`    | `2`   | The maximum depth to visit links |
### Running

Run the Go application

```sh
make run
```

Or with docker:

```sh
make docker-run
```

## Formatting

```sh
make tidy
```

## Testing

Run all unit-tests:

```sh
make test
```