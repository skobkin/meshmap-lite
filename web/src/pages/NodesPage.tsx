import type { NodeDetails } from '../api/types'
import { relativeTime } from '../utils/time'

interface Props {
  items: Array<{ node_id: string; display_name: string; role?: string }>
  selected?: string
  details?: NodeDetails
  loadError?: string
  onSelect: (id: string) => void
}

function value(v: string | number | boolean | undefined): string {
  if (typeof v === 'boolean') return v ? 'yes' : 'no'
  if (typeof v === 'number') return String(v)
  return v && v.length > 0 ? v : 'n/a'
}

export function NodesPage({ items, selected, details, loadError, onSelect }: Props) {
  return (
    <section className="nodes-layout container-fluid">
      <article className="node-list">
        {loadError && <p className="load-error">{loadError}</p>}
        {items.map((n) => (
          <a key={n.node_id} href="#" className={selected === n.node_id ? 'active' : ''} onClick={(e) => { e.preventDefault(); onSelect(n.node_id) }}>
            <strong>{n.display_name}</strong>
          </a>
        ))}
      </article>
      <article className="node-details">
        {details ? (
          <>
            <h3>{details.node.long_name ?? details.node.short_name ?? details.node.node_id}</h3>
            <section>
              <h4>Identity</h4>
              <p>ID: <code>{details.node.node_id}</code></p>
              <p>Long name: {value(details.node.long_name)}</p>
              <p>Short name: {value(details.node.short_name)}</p>
              <p>Node num: {value(details.node.node_num)}</p>
              <p>Role: {value(details.node.role)}</p>
            </section>
            <section>
              <h4>Connectivity / Last Seen</h4>
              <p>MQTT gateway capable: {value(details.node.mqtt_gateway_capable)}</p>
              <p>Last MQTT seen: {relativeTime(details.node.last_seen_mqtt_gateway_at)}</p>
              <p>Last any event: {relativeTime(details.node.last_seen_any_event_at)}</p>
              <p>Last update write: {relativeTime(details.node.updated_at)}</p>
              <p>First seen: {relativeTime(details.node.first_seen_at)}</p>
            </section>
            <section>
              <h4>LoRa / Radio</h4>
              <p>Region: {value(details.node.lora_region)}</p>
              <p>Frequency: {value(details.node.lora_frequency_desc)}</p>
              <p>Modem preset: {value(details.node.modem_preset)}</p>
              <p>Default channel: {value(details.node.has_default_channel)}</p>
              <p>Location reports opted-in: {value(details.node.has_opted_report_location)}</p>
              <p>Neighbors: {value(details.node.neighbor_nodes_count)}</p>
              <p>Board model: {value(details.node.board_model)}</p>
              <p>Firmware: {value(details.node.firmware_version)}</p>
            </section>
            <section>
              <h4>Position</h4>
              <p>Latitude: {value(details.position?.latitude)}</p>
              <p>Longitude: {value(details.position?.longitude)}</p>
              <p>Altitude (m): {value(details.position?.altitude_m)}</p>
              <p>Source kind: {value(details.position?.source_kind)}</p>
              <p>Source channel: {value(details.position?.source_channel)}</p>
              <p>Reported at: {relativeTime(details.position?.reported_at)}</p>
              <p>Observed at: {relativeTime(details.position?.observed_at)}</p>
              <p>Last position update: {relativeTime(details.node.last_seen_position_at)}</p>
            </section>
            <section>
              <h4>Telemetry</h4>
              <p>Voltage: {value(details.telemetry?.power?.voltage)}</p>
              <p>Battery level: {value(details.telemetry?.power?.battery_level)}</p>
              <p>Temperature (C): {value(details.telemetry?.environment?.temperature_c)}</p>
              <p>Humidity: {value(details.telemetry?.environment?.humidity)}</p>
              <p>Pressure (hPa): {value(details.telemetry?.environment?.pressure_hpa)}</p>
              <p>PM2.5: {value(details.telemetry?.air_quality?.pm25)}</p>
              <p>PM10: {value(details.telemetry?.air_quality?.pm10)}</p>
              <p>CO2: {value(details.telemetry?.air_quality?.co2)}</p>
              <p>IAQ: {value(details.telemetry?.air_quality?.iaq)}</p>
            </section>
            <section>
              <h4>Source / Timestamps</h4>
              <p>Telemetry source channel: {value(details.telemetry?.source_channel)}</p>
              <p>Telemetry reported at: {relativeTime(details.telemetry?.reported_at)}</p>
              <p>Telemetry observed at: {relativeTime(details.telemetry?.observed_at)}</p>
              <p>Telemetry updated at: {relativeTime(details.telemetry?.updated_at)}</p>
            </section>
          </>
        ) : <p>Select node</p>}
      </article>
    </section>
  )
}
