package meshtastic

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
	"time"
)

type realWorldChannelSample struct {
	Name              string  `json:"name"`
	Topic             string  `json:"topic"`
	ChannelPSK        string  `json:"channel_psk"`
	PayloadHex        string  `json:"payload_hex"`
	ExpectedKind      string  `json:"expected_kind"`
	ExpectedNodeID    string  `json:"expected_node_id"`
	ExpectedGatewayID string  `json:"expected_gateway_id"`
	ExpectedPacketID  uint32  `json:"expected_packet_id"`
	ExpectedTimestamp string  `json:"expected_timestamp"`
	ExpectedLatitude  float64 `json:"expected_latitude"`
	ExpectedLongitude float64 `json:"expected_longitude"`
	ExpectedAltitudeM float64 `json:"expected_altitude_m"`
}

type realWorldChatSample struct {
	Name              string `json:"name"`
	Topic             string `json:"topic"`
	ChannelPSK        string `json:"channel_psk"`
	PayloadHex        string `json:"payload_hex"`
	ExpectedNodeID    string `json:"expected_node_id"`
	ExpectedGatewayID string `json:"expected_gateway_id"`
	ExpectedPacketID  uint32 `json:"expected_packet_id"`
	ExpectedTimestamp string `json:"expected_timestamp"`
	ExpectedText      string `json:"expected_text"`
}

type realWorldNodeInfoSample struct {
	Name                string `json:"name"`
	Topic               string `json:"topic"`
	ChannelPSK          string `json:"channel_psk"`
	PayloadHex          string `json:"payload_hex"`
	ExpectedNodeID      string `json:"expected_node_id"`
	ExpectedGatewayID   string `json:"expected_gateway_id"`
	ExpectedPacketID    uint32 `json:"expected_packet_id"`
	ExpectedTimestamp   string `json:"expected_timestamp"`
	ExpectedLongName    string `json:"expected_long_name"`
	ExpectedShortName   string `json:"expected_short_name"`
	ExpectedRole        string `json:"expected_role"`
	ExpectedBoardModel  string `json:"expected_board_model"`
	ExpectedFirmware    string `json:"expected_firmware_version"`
	ExpectedLoRaRegion  string `json:"expected_lora_region"`
	ExpectedModemPreset string `json:"expected_modem_preset"`
}

type realWorldTelemetrySample struct {
	Name               string  `json:"name"`
	Topic              string  `json:"topic"`
	ChannelPSK         string  `json:"channel_psk"`
	PayloadHex         string  `json:"payload_hex"`
	ExpectedNodeID     string  `json:"expected_node_id"`
	ExpectedGatewayID  string  `json:"expected_gateway_id"`
	ExpectedPacketID   uint32  `json:"expected_packet_id"`
	ExpectedTimestamp  string  `json:"expected_timestamp"`
	ExpectedVoltage    float64 `json:"expected_voltage"`
	ExpectedBatteryPct float64 `json:"expected_battery_level"`
}

type realWorldMapReportSample struct {
	Name                       string  `json:"name"`
	Topic                      string  `json:"topic"`
	PayloadHex                 string  `json:"payload_hex"`
	ExpectedNodeID             string  `json:"expected_node_id"`
	ExpectedLongName           string  `json:"expected_long_name"`
	ExpectedShortName          string  `json:"expected_short_name"`
	ExpectedRole               string  `json:"expected_role"`
	ExpectedBoardModel         string  `json:"expected_board_model"`
	ExpectedFirmware           string  `json:"expected_firmware_version"`
	ExpectedLoRaRegion         string  `json:"expected_lora_region"`
	ExpectedModemPreset        string  `json:"expected_modem_preset"`
	ExpectedHasDefaultChannel  bool    `json:"expected_has_default_channel"`
	ExpectedHasOptedLocation   bool    `json:"expected_has_opted_report_location"`
	ExpectedNeighborNodesCount int     `json:"expected_neighbor_nodes_count"`
	ExpectedLatitude           float64 `json:"expected_latitude"`
	ExpectedLongitude          float64 `json:"expected_longitude"`
	ExpectedAltitudeM          float64 `json:"expected_altitude_m"`
	ExpectedPositionPrecision  uint32  `json:"expected_position_precision"`
}

func readFixtureFile(t *testing.T, name string) []byte {
	t.Helper()

	switch name {
	case "real_world_position_samples.json":
		data, err := os.ReadFile("testdata/real_world_position_samples.json")
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}

		return data
	case "real_world_chat_samples.json":
		data, err := os.ReadFile("testdata/real_world_chat_samples.json")
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}

		return data
	case "real_world_nodeinfo_samples.json":
		data, err := os.ReadFile("testdata/real_world_nodeinfo_samples.json")
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}

		return data
	case "real_world_telemetry_samples.json":
		data, err := os.ReadFile("testdata/real_world_telemetry_samples.json")
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}

		return data
	case "real_world_mapreport_samples.json":
		data, err := os.ReadFile("testdata/real_world_mapreport_samples.json")
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}

		return data
	default:
		t.Fatalf("unsupported fixture file: %s", name)
	}

	return nil
}

func loadRealWorldChannelSamples(t *testing.T, name string) []realWorldChannelSample {
	t.Helper()

	data := readFixtureFile(t, name)

	var samples []realWorldChannelSample
	if err := json.Unmarshal(data, &samples); err != nil {
		t.Fatalf("decode %s: %v", name, err)
	}

	return samples
}

func loadRealWorldChatSamples(t *testing.T, name string) []realWorldChatSample {
	t.Helper()

	data := readFixtureFile(t, name)

	var samples []realWorldChatSample
	if err := json.Unmarshal(data, &samples); err != nil {
		t.Fatalf("decode %s: %v", name, err)
	}

	return samples
}

func loadRealWorldNodeInfoSamples(t *testing.T, name string) []realWorldNodeInfoSample {
	t.Helper()

	data := readFixtureFile(t, name)

	var samples []realWorldNodeInfoSample
	if err := json.Unmarshal(data, &samples); err != nil {
		t.Fatalf("decode %s: %v", name, err)
	}

	return samples
}

func loadRealWorldTelemetrySamples(t *testing.T, name string) []realWorldTelemetrySample {
	t.Helper()

	data := readFixtureFile(t, name)

	var samples []realWorldTelemetrySample
	if err := json.Unmarshal(data, &samples); err != nil {
		t.Fatalf("decode %s: %v", name, err)
	}

	return samples
}

func loadRealWorldMapReportSamples(t *testing.T, name string) []realWorldMapReportSample {
	t.Helper()

	data := readFixtureFile(t, name)

	var samples []realWorldMapReportSample
	if err := json.Unmarshal(data, &samples); err != nil {
		t.Fatalf("decode %s: %v", name, err)
	}

	return samples
}

func mustDecodeFixtureHex(t *testing.T, value string) []byte {
	t.Helper()

	payload, err := hex.DecodeString(value)
	if err != nil {
		t.Fatalf("decode hex payload: %v", err)
	}

	return payload
}

func mustParseFixtureTimestamp(t *testing.T, value string) time.Time {
	t.Helper()

	ts, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatalf("parse timestamp %q: %v", value, err)
	}

	return ts
}
