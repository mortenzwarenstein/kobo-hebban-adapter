# kobo-hebban-adapter

A lightweight proxy that sits between a Kobo e-reader and the official Kobo store API, automatically syncing reading progress to [Hebban](https://www.hebban.nl).

## How it works

Configure your Kobo to point its API endpoint at this service instead of `storeapi.kobo.com`. The adapter transparently forwards all traffic to Kobo, while intercepting two endpoints:

- `GET /{token}/v1/library/sync` — caches book metadata (title, author) by entitlement ID
- `PUT /{token}/v1/library/{book_id}/state` — reads the Kobo status and updates the book on Hebban

The Kobo device always gets its response first; Hebban sync happens asynchronously and never adds latency.

Kobo status is mapped to Hebban status as follows:

| Kobo status | Hebban status |
|-------------|---------------|
| `Reading`   | `reading`     |
| `Finished`  | `read`        |
| anything else | (skipped)   |

Requests with an unrecognised token are still proxied transparently to Kobo — no Hebban sync will happen.
Any request under a token that isn't a Kobo `/v1/...` path is rejected with a 404 instead of being forwarded.

## Multi-tenant setup

Each user gets a unique secret token that forms their personal URL prefix. Users never share tokens or Hebban credentials.

### Adding users

In production, edit the gitignored `config.json` at
`cicd/apps/kobo-hebban-adapter/overlays/prod/secrets/config.json` (see the `cicd` repo). Locally:

```json
{
  "port": "8080",
  "users": {
    "generated-token-for-alice": {
      "name": "Alice",
      "hebbanToken": "her-hebban-jwt"
    },
    "generated-token-for-bob": {
      "name": "Bob",
      "hebbanToken": "his-hebban-jwt"
    }
  }
}
```

`port` is optional and defaults to `8080`.

Generate a token with:

```sh
openssl rand -hex 32
```

Get your Hebban JWT by logging in to [hebban.nl](https://www.hebban.nl) and copying the `hebban-authorization-token` cookie value from your browser's dev tools.

### Pointing your Kobo at the adapter

On your Kobo device, edit `.kobo/Kobo/Kobo eReader.conf` and set:

```ini
[OneStoreServices]
api_endpoint=https://hebban.mortenzwarenstein.nl/your-token
```

The device and the adapter must be able to reach each other over the network. The adapter itself needs outbound internet access to reach `storeapi.kobo.com` and `www.hebban.nl`.

## Running locally

Create a `config.json` file:

```json
{
  "port": "8080",
  "users": {
    "my-local-token": {
      "name": "Me",
      "hebbanToken": "your-hebban-jwt"
    }
  }
}
```

Then run:

```sh
CONFIG_PATH=config.json go run ./cmd
```

## Deployment

The service runs on Kubernetes at `https://hebban.mortenzwarenstein.nl`. The Kustomize manifests and ArgoCD
`Application` live in the `cicd` repo, not here (`cicd/apps/kobo-hebban-adapter/` and
`cicd/argocd/kobo-hebban-adapter.yaml`). ArgoCD syncs the cluster continuously from `cicd`.

This repo only builds and pushes the image, on every published GitHub release:
1. Builds and pushes `ghcr.io/mortenzwarenstein/kobo-hebban-adapter:<release-tag>` to GHCR (private)

To actually roll out a release, bump `images[].newTag` in
`cicd/apps/kobo-hebban-adapter/overlays/prod/kustomization.yaml` to the new tag and commit — ArgoCD picks up
the change and syncs it.

**Requires:**
- The GHCR package `kobo-hebban-adapter` set to private (one-time, in GitHub package settings).
- A `ghcr-pull-secret` (`kubernetes.io/dockerconfigjson`, scope `read:packages`) in the `kobo` namespace, created
  out of band — it's what lets the cluster pull the private image.
