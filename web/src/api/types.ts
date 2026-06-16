export type WSState = 'connecting' | 'connected' | 'reconnecting' | 'disconnected'
export type MQTTConnectionStatus = 'connected' | 'disconnected'
export type MapPrecisionCirclesMode = 'none' | 'selected' | 'always'

export interface WSHeartbeat {
  status: 'ok'
  mqtt_connection_status: MQTTConnectionStatus
}

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
  node_num?: number
  long_name?: string
  short_name?: string
  role?: string
  board_model?: string
  firmware_version?: string
  lora_region?: string
  lora_frequency_desc?: string
  modem_preset?: string
  has_default_channel?: boolean
  has_opted_report_location?: boolean
  neighbor_nodes_count?: number
  mqtt_gateway_capable?: boolean
  first_seen_at?: string
  last_seen_any_event_at: string
  last_seen_mqtt_gateway_at?: string
  last_mqtt_uploader_node_id?: string
  last_mqtt_uploader_display_name?: string
  last_mqtt_uploader_at?: string
  last_seen_position_at?: string
  updated_at?: string
}

export interface NodePosition {
  node_id: string
  latitude: number
  longitude: number
  altitude_m?: number
  position_precision?: number
  source_kind: string
  source_channel?: string
  mqtt_uploader_node_id?: string
  mqtt_uploader_display_name?: string
  reported_at?: string
  observed_at: string
  updated_at?: string
}

export interface MapNode {
  node: Node
  position?: NodePosition
  telemetry?: NodeTelemetry
}

export interface ChatEvent {
  id: number
  event_type: 'message' | 'system' | 'reaction'
  channel_name?: string
  node_id?: string
  node_display_name?: string
  mqtt_uploader_node_id?: string
  mqtt_uploader_display_name?: string
  system_code?: string
  message_text?: string
  reaction_emoji?: string
  reply_to_packet_id?: number
  observed_at: string
  packet_id?: number
  hop_start?: number
  hop_limit?: number
}

export interface NodeSummary {
  node_id: string
  display_name: string
  long_name?: string
  short_name?: string
  last_seen_any_event_at: string
  last_seen_position_at?: string
  last_seen_mqtt_gateway_at?: string
  last_mqtt_uploader_node_id?: string
  last_mqtt_uploader_display_name?: string
  last_mqtt_uploader_at?: string
  has_position: boolean
  role?: string
  board_model?: string
}

export interface NodeDetails {
  node: Node
  position?: NodePosition
  telemetry?: NodeTelemetry
  neighbors?: NodeNeighbor[]
  previous_names?: NodeNameHistory[]
}

export interface NodeNameHistory {
  previous_long_name?: string
  previous_short_name?: string
  new_long_name?: string
  new_short_name?: string
  changed_at: string
}

export interface TopologyEdge {
  source_kind: 'neighbor_info' | 'routing_forward' | 'routing_return' | 'traceroute_forward' | 'traceroute_return' | 'mqtt_direct'
  channel_name?: string
  from_node_id: string
  to_node_id: string
  reported_by_node_id?: string
  inferred?: boolean
  snr?: number
  neighbor_last_rx_at?: string
  neighbor_broadcast_interval_secs?: number
  first_observed_at: string
  last_observed_at: string
  last_reported_at?: string
  updated_at: string
}

export interface TopologyEdgesResponse {
  items: TopologyEdge[]
  truncated: boolean
}

export interface NodeNeighbor {
  node_id: string
  display_name: string
  long_name?: string
  short_name?: string
  has_position: boolean
  evidence_kind: 'neighbor_info' | 'mqtt_direct' | 'inferred'
  snr?: number
  channel_name?: string
  reported_by_node_id?: string
  neighbor_last_rx_at?: string
  neighbor_broadcast_interval_secs?: number
  last_observed_at: string
  last_reported_at?: string
  updated_at: string
}

export interface NodeTelemetry {
  node_id: string
  power: {
    voltage?: number
    battery_level?: number
  }
  environment: {
    temperature_c?: number
    humidity?: number
    pressure_hpa?: number
  }
  air_quality: {
    pm25?: number
    pm10?: number
    co2?: number
    iaq?: number
  }
  source_channel?: string
  mqtt_uploader_node_id?: string
  mqtt_uploader_display_name?: string
  reported_at?: string
  observed_at: string
  updated_at: string
}

export interface Meta {
  app_name: string
  version: string
  websocket_path: string
  default_chat_channel: string
  show_recent_messages: number
  log_live_updates: boolean
  log_page_size_default: number
  disconnected_threshold: string
  info_available: boolean
  info_source_hash?: string
  update_check_available?: boolean
  update_check_sources?: SourceSummary[]
  relevance: {
    telemetry_max_age: string
    topology_evidence_max_age: string
    map_position_max_age: string
  }
  map: {
    clustering: boolean
    topology_cache_ttl: string
    precision_circles_mode: MapPrecisionCirclesMode
    default_view: {
      latitude: number
      longitude: number
      zoom: number
    }
  }
}

export type InfoFormat = 'html' | 'markdown'

export interface InfoResponse {
  format: InfoFormat
  source_hash: string
  content: string
}

export interface SourceSummaryRelease {
  version: string
  published_at: string
  prerelease: boolean
}

export interface SourceSummary {
  name: string
  label: string
  releases_page_url?: string
  source_hash?: string
  current_version?: string
  latest_version?: string
  update_available?: boolean
  releases: SourceSummaryRelease[]
}

export interface UpdateRelease {
  version: string
  published_at: string
  html_url: string
  body: string
  prerelease: boolean
}

export interface UpdatesResponse {
  format: 'html' | 'markdown'
  source: string
  source_hash: string
  releases: UpdateRelease[]
}

export interface ActivityBucket {
  bucket_start: string
  text_messages: number
  pki: number
  node_info: number
  telemetry: number
  neighbor_info: number
  range_test: number
  traceroute: number
}

export interface ActivityPeriod {
  key: 'daily' | 'weekly'
  title: string
  window: string
  bucket: string
  buckets: ActivityBucket[]
}

export interface ActivityStats {
  generated_at: string
  periods: ActivityPeriod[]
}

export interface LogEvent {
  id: number
  observed_at: string
  node_id?: string
  node_display_name?: string
  mqtt_uploader_node_id?: string
  mqtt_uploader_display_name?: string
  event_kind_value: number
  event_kind_title: string
  encrypted: boolean
  channel_name?: string | null
  details?: Record<string, unknown>
  hop_start?: number
  hop_limit?: number
}
