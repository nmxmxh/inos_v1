//go:build js && wasm
// +build js,wasm

package main

import (
	"syscall/js"
	"time"

	"github.com/nmxmxh/inos_v1/kernel/core/mesh/transport"
	"github.com/nmxmxh/inos_v1/kernel/utils"
)

type MeshIdentity struct {
	DID         string
	DeviceID    string
	NodeID      string
	DisplayName string
}

type MeshBootstrapConfig struct {
	Identity  MeshIdentity
	Region    string
	Transport transport.TransportConfig
}

func loadMeshConfig() MeshBootstrapConfig {
	config := MeshBootstrapConfig{
		Region:    "global",
		Transport: transport.DefaultTransportConfig(),
		Identity: MeshIdentity{
			DID:         "did:inos:system",
			DeviceID:    "device:unknown",
			NodeID:      "node:" + utils.GenerateID(),
			DisplayName: "Guest",
		},
	}

	global := js.Global()
	rawConfig := global.Get("__INOS_MESH_CONFIG__")
	if !rawConfig.IsUndefined() && !rawConfig.IsNull() {
		if region := rawConfig.Get("region"); region.Type() == js.TypeString {
			config.Region = region.String()
		}

		// Diagnostic: check for WebRTC
		pc := global.Get("RTCPeerConnection")
		if pc.IsUndefined() {
			utils.DefaultLogger("kernel").Warn("RTCPeerConnection IS UNDEFINED in this context")
		} else {
			utils.DefaultLogger("kernel").Info("RTCPeerConnection is available")
		}

		if identity := rawConfig.Get("identity"); identity.Type() == js.TypeObject {
			if did := identity.Get("did"); did.Type() == js.TypeString {
				config.Identity.DID = did.String()
			}
			if deviceID := identity.Get("deviceId"); deviceID.Type() == js.TypeString {
				config.Identity.DeviceID = deviceID.String()
			}
			if nodeID := identity.Get("nodeId"); nodeID.Type() == js.TypeString {
				config.Identity.NodeID = nodeID.String()
			}
			if displayName := identity.Get("displayName"); displayName.Type() == js.TypeString {
				config.Identity.DisplayName = displayName.String()
			}
		}

		if transportCfg := rawConfig.Get("transport"); transportCfg.Type() == js.TypeObject {
			applyTransportConfigOverrides(&config.Transport, transportCfg)
		}
	}

	rawIdentity := global.Get("__INOS_IDENTITY__")
	if !rawIdentity.IsUndefined() && !rawIdentity.IsNull() {
		if did := rawIdentity.Get("did"); did.Type() == js.TypeString {
			config.Identity.DID = did.String()
		}
		if deviceID := rawIdentity.Get("deviceId"); deviceID.Type() == js.TypeString {
			config.Identity.DeviceID = deviceID.String()
		}
		if nodeID := rawIdentity.Get("nodeId"); nodeID.Type() == js.TypeString {
			config.Identity.NodeID = nodeID.String()
		}
		if displayName := rawIdentity.Get("displayName"); displayName.Type() == js.TypeString {
			config.Identity.DisplayName = displayName.String()
		}
	}

	if config.Identity.NodeID == "" {
		config.Identity.NodeID = "node:" + utils.GenerateID()
	}

	if len(config.Transport.SignalingServers) == 0 && config.Transport.WebSocketURL != "" {
		config.Transport.SignalingServers = []string{config.Transport.WebSocketURL}
	}
	if config.Transport.WebSocketURL == "" && len(config.Transport.SignalingServers) > 0 {
		config.Transport.WebSocketURL = config.Transport.SignalingServers[0]
	}

	utils.Info("Mesh config loaded",
		utils.String("node_id", config.Identity.NodeID),
		utils.String("signaling_url", config.Transport.WebSocketURL),
		utils.Any("signaling_servers", config.Transport.SignalingServers),
	)

	return config
}

func applyTransportConfigOverrides(cfg *transport.TransportConfig, raw js.Value) {
	if v := raw.Get("webrtcEnabled"); v.Type() == js.TypeBoolean {
		cfg.WebRTCEnabled = v.Bool()
	}
	if v := raw.Get("webSocketUrl"); v.Type() == js.TypeString {
		cfg.WebSocketURL = v.String()
	}
	if v := raw.Get("iceServers"); v.Type() == js.TypeObject {
		cfg.ICEServers = readStringSlice(v)
	}
	if v := raw.Get("stunServers"); v.Type() == js.TypeObject {
		cfg.STUNServers = readStringSlice(v)
	}
	if v := raw.Get("turnServers"); v.Type() == js.TypeObject {
		cfg.TURNServers = readStringSlice(v)
	}
	if v := raw.Get("signalingServers"); v.Type() == js.TypeObject {
		cfg.SignalingServers = readStringSlice(v)
	}
	if v := raw.Get("maxConnections"); v.Type() == js.TypeNumber {
		cfg.MaxConnections = v.Int()
	}
	if v := raw.Get("maxMessageSize"); v.Type() == js.TypeNumber {
		cfg.MaxMessageSize = v.Int()
	}
	if v := raw.Get("poolSize"); v.Type() == js.TypeNumber {
		cfg.PoolSize = v.Int()
	}
	if v := raw.Get("connectionTimeoutMs"); v.Type() == js.TypeNumber {
		cfg.ConnectionTimeout = time.Duration(v.Int()) * time.Millisecond
	}
	if v := raw.Get("reconnectDelayMs"); v.Type() == js.TypeNumber {
		cfg.ReconnectDelay = time.Duration(v.Int()) * time.Millisecond
	}
	if v := raw.Get("keepAliveIntervalMs"); v.Type() == js.TypeNumber {
		cfg.KeepAliveInterval = time.Duration(v.Int()) * time.Millisecond
	}
	if v := raw.Get("rpcTimeoutMs"); v.Type() == js.TypeNumber {
		cfg.RPCTimeout = time.Duration(v.Int()) * time.Millisecond
	}
	if v := raw.Get("poolMaxIdleMs"); v.Type() == js.TypeNumber {
		cfg.PoolMaxIdle = time.Duration(v.Int()) * time.Millisecond
	}
	if v := raw.Get("metricsIntervalMs"); v.Type() == js.TypeNumber {
		cfg.MetricsInterval = time.Duration(v.Int()) * time.Millisecond
	}
	if v := raw.Get("maxRetries"); v.Type() == js.TypeNumber {
		cfg.MaxRetries = v.Int()
	}
}

func readStringSlice(val js.Value) []string {
	if val.IsUndefined() || val.IsNull() {
		return nil
	}
	length := val.Length()
	out := make([]string, 0, length)
	for i := 0; i < length; i++ {
		item := val.Index(i)
		if item.Type() == js.TypeString {
			out = append(out, item.String())
		}
	}
	return out
}
