# kobo-hebban-adapter

A lightweight proxy that sits between a Kobo e-reader and the official Kobo store API, automatically syncing reading progress to [Hebban](https://www.hebban.nl).

## How it works

Configure your Kobo to point its API endpoint at this service instead of `storeapi.kobo.com`. The adapter transparently forwards all traffic to Kobo, while intercepting two endpoints:

- `GET /v1/library/sync` — caches book metadata (title, author) by entitlement ID
- `PUT /v1/library/{book_id}/state` — reads progress from the payload and updates the book's status on Hebban

The Kobo device always gets its response first; Hebban sync happens asynchronously and never adds latency.

Progress is mapped to Hebban status as follows:

| Progress | Status |
|----------|--------|
| 0% | (skipped) |
| 1–99% | `reading` |
| 100% | `read` |

## Configuration

| Environment variable | Required | Description |
|---|---|---|
| `HEBBAN_AUTH_TOKEN` | Yes | Value of the `hebban-authorization-token` cookie from your browser session |
| `PORT` | No | Port to listen on (default: `8080`) |

## Pointing your Kobo at the adapter

On your Kobo device, set the API endpoint to your adapter's address. This is typically done by editing `.kobo/Kobo/Kobo eReader.conf` on the device and setting:

```ini
[OneStoreServices]
api_endpoint=https://hebban.mortenzwarenstein.nl
```

The device and the adapter must be able to reach each other over the network. The adapter itself needs outbound internet access to reach `storeapi.kobo.com` and `www.hebban.nl`.

## Running locally

```sh
HEBBAN_AUTH_TOKEN=your-token go run .
```

## Deployment

The service runs on Kubernetes. Deployment is handled automatically by GitHub Actions on every push to `main`.

The pipeline:
1. Builds and pushes `ghcr.io/mortenzwarenstein/kobo-hebban-adapter:latest` to GHCR
2. Applies the kustomize manifests to the cluster
3. Rolls out the new deployment

**GitHub Actions secrets required:**

| Secret | Description |
|---|---|
| `HEBBAN_AUTH_TOKEN` | Your Hebban auth token |
| `KUBECONFIG_PROD` | Kubeconfig for the production cluster |

The service is exposed at `https://hebban.mortenzwarenstein.nl` via Traefik with a Let's Encrypt certificate.
