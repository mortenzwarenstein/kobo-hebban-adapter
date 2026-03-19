FROM golang:1.24-alpine AS build

WORKDIR /src
COPY go.mod ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /kobo-hebban-adapter .

# ── runtime ──────────────────────────────────────────────────────────────────
FROM scratch

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /kobo-hebban-adapter /kobo-hebban-adapter

EXPOSE 8080
ENTRYPOINT ["/kobo-hebban-adapter"]
