// Package store provides DuckDB persistence for orb-collector.
package store

// DDL defines the database schema.
const DDL = `
CREATE SEQUENCE IF NOT EXISTS seq_tests_id START 1;
CREATE SEQUENCE IF NOT EXISTS seq_responsiveness_id START 1;
CREATE SEQUENCE IF NOT EXISTS seq_wifi_link_id START 1;
CREATE SEQUENCE IF NOT EXISTS seq_ap_snapshots_id START 1;
CREATE SEQUENCE IF NOT EXISTS seq_speed_results_id START 1;
CREATE SEQUENCE IF NOT EXISTS seq_scores_id START 1;

CREATE TABLE IF NOT EXISTS tests (
    id              BIGINT DEFAULT nextval('seq_tests_id') PRIMARY KEY,
    name            TEXT NOT NULL,
    channel         INTEGER NOT NULL,
    width_mhz       INTEGER NOT NULL,
    freq_mhz        INTEGER NOT NULL,
    start_time      TIMESTAMP NOT NULL,
    end_time        TIMESTAMP,
    ap_platform     TEXT,
    ap_name         TEXT,
    ap_rssi         INTEGER,
    ap_snr          INTEGER,
    ap_channel      INTEGER,
    ap_power_save   BOOLEAN,
    ap_rx_rate      INTEGER,
    ap_tx_rate      INTEGER,
    ap_wifi_mode    TEXT,
    notes           TEXT
);

CREATE TABLE IF NOT EXISTS responsiveness (
    id                      BIGINT DEFAULT nextval('seq_responsiveness_id') PRIMARY KEY,
    test_id                 BIGINT REFERENCES tests(id),
    orb_device_id           TEXT NOT NULL DEFAULT '',
    collected_at            TIMESTAMP NOT NULL,
    orb_timestamp           TIMESTAMP NOT NULL,
    interval_ms             INTEGER,
    network_name            TEXT,
    bssid                   TEXT,
    router_latency_avg_us   BIGINT,
    router_latency_count    INTEGER,
    router_latency_lost     INTEGER,
    router_lag_avg_us       BIGINT,
    router_lag_count        INTEGER,
    router_jitter_avg_us    BIGINT,
    router_packet_loss_pct  DOUBLE,
    latency_avg_us          BIGINT,
    latency_count           INTEGER,
    latency_lost_count      INTEGER,
    lag_avg_us              BIGINT,
    lag_count               INTEGER,
    jitter_avg_us           BIGINT,
    packet_loss_pct         DOUBLE,
    raw                     JSON NOT NULL,
    UNIQUE(orb_device_id, orb_timestamp)
);

CREATE TABLE IF NOT EXISTS wifi_link (
    id              BIGINT DEFAULT nextval('seq_wifi_link_id') PRIMARY KEY,
    test_id         BIGINT REFERENCES tests(id),
    orb_device_id   TEXT NOT NULL DEFAULT '',
    collected_at    TIMESTAMP NOT NULL,
    orb_timestamp   TIMESTAMP NOT NULL,
    interval_ms     INTEGER,
    network_name    TEXT,
    bssid           TEXT,
    rssi_avg        INTEGER,
    snr_avg         INTEGER,
    noise_avg       INTEGER,
    tx_rate_mbps    INTEGER,
    rx_rate_mbps    INTEGER,
    frequency_mhz   INTEGER,
    channel_number  INTEGER,
    channel_band    TEXT,
    channel_width   TEXT,
    phy_mode        TEXT,
    raw             JSON NOT NULL,
    UNIQUE(orb_device_id, orb_timestamp)
);

CREATE TABLE IF NOT EXISTS speed_results (
    id              BIGINT DEFAULT nextval('seq_speed_results_id') PRIMARY KEY,
    test_id         BIGINT REFERENCES tests(id),
    orb_device_id   TEXT NOT NULL DEFAULT '',
    collected_at    TIMESTAMP NOT NULL,
    orb_timestamp   TIMESTAMP NOT NULL,
    network_name    TEXT,
    bssid           TEXT,
    download_kbps   BIGINT,
    upload_kbps     BIGINT,
    download_bytes  BIGINT,
    upload_bytes    BIGINT,
    server_name     TEXT,
    raw             JSON NOT NULL,
    UNIQUE(orb_device_id, orb_timestamp)
);

CREATE TABLE IF NOT EXISTS scores (
    id                      BIGINT DEFAULT nextval('seq_scores_id') PRIMARY KEY,
    test_id                 BIGINT REFERENCES tests(id),
    orb_device_id           TEXT NOT NULL DEFAULT '',
    collected_at            TIMESTAMP NOT NULL,
    orb_timestamp           TIMESTAMP NOT NULL,
    interval_ms             INTEGER,
    network_name            TEXT,
    bssid                   TEXT,
    orb_score               DOUBLE,
    responsiveness_score    DOUBLE,
    reliability_score       DOUBLE,
    speed_score             DOUBLE,
    lag_avg_us              BIGINT,
    lag_count               INTEGER,
    download_avg_kbps       BIGINT,
    upload_avg_kbps         BIGINT,
    speed_age_ms            BIGINT,
    speed_count             INTEGER,
    unresponsive_ms         BIGINT,
    measured_ms             BIGINT,
    raw                     JSON NOT NULL,
    UNIQUE(orb_device_id, orb_timestamp)
);

CREATE TABLE IF NOT EXISTS ap_snapshots (
    id              BIGINT DEFAULT nextval('seq_ap_snapshots_id') PRIMARY KEY,
    test_id         BIGINT REFERENCES tests(id),
    client_mac      TEXT NOT NULL DEFAULT '',
    timestamp       TIMESTAMP NOT NULL,
    platform        TEXT,
    rssi            INTEGER,
    snr             INTEGER,
    signal          INTEGER,
    channel         INTEGER,
    band            TEXT,
    ap_name         TEXT,
    power_save      BOOLEAN,
    rx_rate         INTEGER,
    tx_rate         INTEGER,
    wifi_mode       TEXT,
    activity        INTEGER,
    raw             JSON NOT NULL
);
`
