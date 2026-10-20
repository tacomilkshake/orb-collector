FROM docker.io/library/golang:1.27-bookworm@sha256:5c3e9b43cfd3bb95c7937462e29d87ada57b481a96fa23ad0eb61778d7a00110 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go test ./...
RUN CGO_ENABLED=1 go build -o /orb-collector .

FROM docker.io/library/debian:bookworm-slim@sha256:7c7b2c966bc9ee8cedfeef67e0e279108992c77681fa595db4a9d65c06ccc587
RUN apt-get update && apt-get upgrade -y && apt-get install -y --no-install-recommends ca-certificates curl procps && rm -rf /var/lib/apt/lists/*
COPY --from=build /orb-collector /usr/local/bin/orb-collector
WORKDIR /data
ENTRYPOINT ["orb-collector"]
CMD ["collect"]
