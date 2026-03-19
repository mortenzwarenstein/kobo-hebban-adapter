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

## Multi-tenant setup

Each user gets a unique secret token that forms their personal URL prefix. Users never share tokens or Hebban credentials.

### Adding users

Edit `users.json`:

```json
{
  "generated-token-for-alice": {
    "name": "Alice",
    "hebbanToken": "her-hebban-jwt"
  },
  "generated-token-for-bob": {
    "name": "Bob",
    "hebbanToken": "his-hebban-jwt"
  }
}
```

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

Create a `users.json` file:

```json
{
  "my-local-token": {
    "name": "Me",
    "hebbanToken": "your-hebban-jwt"
  }
}
```

Then run:

```sh
USERS_CONFIG=users.json go run .
```

## Deployment

The service runs on Kubernetes at `https://hebban.mortenzwarenstein.nl`. Deployment is handled automatically by GitHub Actions on every push to `master`.

The pipeline:
1. Builds and pushes `ghcr.io/mortenzwarenstein/kobo-hebban-adapter:latest` to GHCR
2. Writes `users.json` from the `USERS_JSON` GitHub secret
3. Applies the kustomize manifests to the cluster
4. Rolls out the new deployment

**GitHub Actions secrets required:**

| Secret | Description |
|---|---|
| `USERS_JSON` | Contents of `users.json` |
| `KUBECONFIG_PROD` | Kubeconfig for the production cluster |
