FROM docker.io/library/golang:1.27-trixie@sha256:1896654cfc5c2b5ad74c07f29cb36f5d2d1cbed42e3054070fbfe444e521e83c AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go test ./...
RUN CGO_ENABLED=1 go build -o /orb-collector .

FROM docker.io/library/debian:trixie-slim@sha256:62bd423aeee912639ca8ee681c2b762cb3e25137e0a92b20be455898a9f034ad
RUN apt-get update && apt-get upgrade -y && apt-get install -y --no-install-recommends ca-certificates curl procps && rm -rf /var/lib/apt/lists/*
COPY --from=build /orb-collector /usr/local/bin/orb-collector
WORKDIR /data
ENTRYPOINT ["orb-collector"]
CMD ["collect"]
