export type WSState = 'connecting' | 'connected' | 'reconnecting' | 'disconnected'

export interface WSStats {
  known_nodes_count: number
  online_nodes_count: number
  ws_clients_count: number
  last_ingest_at?: string
}

export interface ChannelItem {
  name: string
  chat_enabled: boolean
  is_primary: boolean
}

export interface Node {
  node_id: string
  long_name?: string
  short_name?: string
  role?: string
  board_model?: string
  firmware_version?: string
  lora_region?: string
  lora_frequency_desc?: string
  modem_preset?: string
  neighbor_nodes_count?: number
  last_seen_any_event_at: string
  last_seen_mqtt_gateway_at?: string
  last_seen_position_at?: string
}

export interface NodePosition {
  node_id: string
  latitude: number
  longitude: number
  altitude_m?: number
  source_kind: string
  source_channel?: string
  observed_at: string
}

export interface MapNode {
  node: Node
  position?: NodePosition
}

export interface ChatEvent {
  id: number
  event_type: 'message' | 'system'
  channel_name?: string
  node_id?: string
  system_code?: string
  message_text?: string
  observed_at: string
}

export interface NodeSummary {
  node_id: string
  display_name: string
  last_seen_any_event_at: string
  last_seen_mqtt_gateway_at?: string
  has_position: boolean
  role?: string
  board_model?: string
}

export interface NodeDetails {
  node: Node
  position?: NodePosition
  telemetry?: Record<string, unknown>
}

export interface Meta {
  websocket_path: string
  default_chat_channel: string
  show_recent_messages: number
  disconnected_threshold: string
  map: {
    default_view: {
      latitude: number
      longitude: number
      zoom: number
    }
  }
}
