// Package orb provides an HTTP client for the Orb local API.
package orb

// ResponsivenessRecord represents a single responsiveness measurement from the Orb.
type ResponsivenessRecord struct {
	Timestamp           int64    `json:"timestamp"`
	IntervalMS          *int     `json:"interval_ms"`
	NetworkName         *string  `json:"network_name"`
	BSSID               *string  `json:"bssid"`
	RouterLatencyAvgUS  *int64   `json:"router_latency_avg_us"`
	RouterLatencyCount  *int     `json:"router_latency_count"`
	RouterLatencyLost   *int     `json:"router_latency_lost_count"`
	RouterLagAvgUS      *int64   `json:"router_lag_avg_us"`
	RouterLagCount      *int     `json:"router_lag_count"`
	RouterJitterAvgUS   *int64   `json:"router_jitter_avg_us"`
	RouterPacketLossPct *float64 `json:"router_packet_loss_pct"`
	LatencyAvgUS        *int64   `json:"latency_avg_us"`
	LatencyCount        *int     `json:"latency_count"`
	LatencyLostCount    *int     `json:"latency_lost_count"`
	LagAvgUS            *int64   `json:"lag_avg_us"`
	LagCount            *int     `json:"lag_count"`
	JitterAvgUS         *int64   `json:"jitter_avg_us"`
	PacketLossPct       *float64 `json:"packet_loss_pct"`
}

// SpeedResultsRecord represents a single speed_results measurement from the Orb.
type SpeedResultsRecord struct {
	Timestamp     int64   `json:"timestamp"`
	IntervalMS    *int    `json:"interval_ms"`
	NetworkName   *string `json:"network_name"`
	BSSID         *string `json:"bssid"`
	DownloadKbps  *int64  `json:"download_kbps"`
	UploadKbps    *int64  `json:"upload_kbps"`
	DownloadBytes *int64  `json:"download_bytes"`
	UploadBytes   *int64  `json:"upload_bytes"`
	ServerName    *string `json:"server_name"`
}

// ScoresRecord represents a single scores measurement from the Orb.
type ScoresRecord struct {
	Timestamp           int64   `json:"timestamp"`
	IntervalMS          *int    `json:"interval_ms"`
	NetworkName         *string `json:"network_name"`
	BSSID               *string `json:"bssid"`
	OrbScore            float64 `json:"orb_score"`
	ResponsivenessScore float64 `json:"responsiveness_score"`
	ReliabilityScore    float64 `json:"reliability_score"`
	SpeedScore          float64 `json:"speed_score"`
	LagAvgUS            *int64  `json:"lag_avg_us"`
	LagCount            *int    `json:"lag_count"`
	DownloadAvgKbps     *int64  `json:"download_avg_kbps"`
	UploadAvgKbps       *int64  `json:"upload_avg_kbps"`
	SpeedAgeMS          *int64  `json:"speed_age_ms"`
	SpeedCount          *int    `json:"speed_count"`
	UnresponsiveMS      *int64  `json:"unresponsive_ms"`
	MeasuredMS          *int64  `json:"measured_ms"`
}

// WifiLinkRecord represents a single wifi_link measurement from the Orb.
type WifiLinkRecord struct {
	Timestamp     int64   `json:"timestamp"`
	IntervalMS    *int    `json:"interval_ms"`
	NetworkName   *string `json:"network_name"`
	BSSID         *string `json:"bssid"`
	RSSIAvg       *int    `json:"rssi_avg"`
	SNRAvg        *int    `json:"snr_avg"`
	NoiseAvg      *int    `json:"noise_avg"`
	TxRateMbps    *int    `json:"tx_rate_mbps"`
	RxRateMbps    *int    `json:"rx_rate_mbps"`
	FrequencyMHz  *int    `json:"frequency_mhz"`
	ChannelNumber *int    `json:"channel_number"`
	ChannelBand   *string `json:"channel_band"`
	ChannelWidth  *string `json:"channel_width"`
	PhyMode       *string `json:"phy_mode"`
}
