# orb-collector

WiFi performance data collector using [Orb](https://orb.net) network sensors and pluggable AP platform connectors.

## What it does

orb-collector continuously collects WiFi performance data from multiple sources:

- **Orb sensors** (1s intervals): responsiveness, WiFi link quality, composite scores, speed tests
- **AP platforms** (30s intervals): all wireless client stats (RSSI, SNR, channel, band, power save, rates)

Data is stored in DuckDB for efficient time-series analytics with native percentile functions.

## Architecture

```
Orb Sensors (Pixel 8a, wlanpi, etc.)
    | HTTP API (1s polling)
    v
+--------------+     +-----------+
| orb-collector|---->|   DuckDB  |
+--------------+     +-----------+
    ^
    | HTTP API (30s polling)
    |
AP Platform (Omada, etc.)
```

## Supported connectors

| Platform      | Status  | Via                                                                    |
| ------------- | ------- | ---------------------------------------------------------------------- |
| TP-Link Omada | Active  | [omada-bridge](https://github.com/tacomilkshake/omada-bridge) REST API |
| Ubiquiti      | Planned | --                                                                     |

## Usage

### Continuous collection (primary mode)

```bash
orb-collector collect \
  --db /data/orb-collector.db \
  --orb-host 10.0.1.47 --orb-port 8000 \
  --ap-connector omada --ap-url http://omada-bridge:8086
```

### Multi-orb collection

```bash
orb-collector collect \
  --db /data/orb-collector.db \
  --orb-hosts "10.0.1.47:8000,10.0.1.48:8000" \
  --ap-connector omada --ap-url http://omada-bridge:8086
```

### Test window management (via HTTP API)

```bash
# While collector is running:
curl -X POST http://localhost:8080/api/begin \
  -d '{"name":"ch53_320mhz","channel":53,"width":320,"freq":6215}'

curl -X POST http://localhost:8080/api/end

curl http://localhost:8080/api/status
```

### CLI commands

```bash
orb-collector status --db /data/orb-collector.db
orb-collector report --db /data/orb-collector.db
orb-collector dump <test-id> --db /data/orb-collector.db
```

## Docker

```bash
docker run -v ./data:/data ghcr.io/tacomilkshake/orb-collector:main \
  collect --db /data/orb-collector.db \
  --orb-host 10.0.1.47 --orb-port 8000 \
  --ap-connector omada --ap-url http://omada-bridge:8086
```

## Data model

Stored in DuckDB with 6 tables:

- **responsiveness** -- per-second latency, lag, jitter, packet loss
- **wifi_link** -- per-second client-side RSSI, SNR, rates, channel
- **scores** -- per-second composite Orb scores (responsiveness, reliability, speed)
- **speed_results** -- periodic throughput tests (download/upload)
- **ap_snapshots** -- all wireless clients from AP platform every 30s
- **tests** -- named test windows for A/B comparison
