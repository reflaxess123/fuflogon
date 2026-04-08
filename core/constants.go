package core

const (
	TunName        = "xray0"
	TunIP          = "198.18.0.1"
	TunNetmask     = "255.255.255.252"
	TunPrefix      = 30
	// 1420 leaves headroom for the upstream link MTU. Many home routers
	// (especially Wi-Fi via PPPoE) cap effective MTU around 1492; with 1500
	// inside the TUN, large TLS handshakes get fragmented or dropped.
	TunMTU = 1420
	RuntimeCfgName = ".xray-runtime.json"
	PidFileName    = ".xray.pid"
	StateFileName  = ".xray-state.json"
	DefaultCfgName = "config.json"
)
