import type { ComponentChildren } from 'preact'
import type { NodeDetails } from '../api/types'
import { relativeTime } from '../utils/time'

interface Props {
  items: Array<{ node_id: string; display_name: string; role?: string }>
  selected?: string
  details?: NodeDetails
  loadError?: string
  onSelect: (id: string) => void
}

interface DetailRow {
  label: string
  value: ComponentChildren
}

interface DetailSection {
  title: string
  rows: DetailRow[]
}

function displayValue(v: string | number | boolean | undefined): string | null {
  if (typeof v === 'boolean') return v ? 'yes' : 'no'
  if (typeof v === 'number') return String(v)
  return v && v.length > 0 ? v : null
}

function displayRelativeTime(v?: string): string | null {
  return v ? relativeTime(v) : null
}

function row(label: string, value: ComponentChildren | null): DetailRow | null {
  return value === null ? null : { label, value }
}

function compactRows(rows: Array<DetailRow | null>): DetailRow[] {
  return rows.filter((item): item is DetailRow => item !== null)
}

function detailSections(details: NodeDetails): DetailSection[] {
  return [
    {
      title: 'Identity',
      rows: compactRows([
        row('ID', <code>{details.node.node_id}</code>),
        row('Long name', displayValue(details.node.long_name)),
        row('Short name', displayValue(details.node.short_name)),
        row('Node num', displayValue(details.node.node_num)),
        row('Role', displayValue(details.node.role))
      ])
    },
    {
      title: 'Connectivity / Last Seen',
      rows: compactRows([
        row('MQTT gateway capable', displayValue(details.node.mqtt_gateway_capable)),
        row('Last MQTT seen', displayRelativeTime(details.node.last_seen_mqtt_gateway_at)),
        row('Last any event', displayRelativeTime(details.node.last_seen_any_event_at)),
        row('Last update write', displayRelativeTime(details.node.updated_at)),
        row('First seen', displayRelativeTime(details.node.first_seen_at))
      ])
    },
    {
      title: 'LoRa / Radio',
      rows: compactRows([
        row('Region', displayValue(details.node.lora_region)),
        row('Frequency', displayValue(details.node.lora_frequency_desc)),
        row('Modem preset', displayValue(details.node.modem_preset)),
        row('Default channel', displayValue(details.node.has_default_channel)),
        row('Location reports opted-in', displayValue(details.node.has_opted_report_location)),
        row('Neighbors', displayValue(details.node.neighbor_nodes_count)),
        row('Board model', displayValue(details.node.board_model)),
        row('Firmware', displayValue(details.node.firmware_version))
      ])
    },
    {
      title: 'Position',
      rows: compactRows([
        row('Latitude', displayValue(details.position?.latitude)),
        row('Longitude', displayValue(details.position?.longitude)),
        row('Altitude (m)', displayValue(details.position?.altitude_m)),
        row('Source kind', displayValue(details.position?.source_kind)),
        row('Source channel', displayValue(details.position?.source_channel)),
        row('Reported at', displayRelativeTime(details.position?.reported_at)),
        row('Observed at', displayRelativeTime(details.position?.observed_at)),
        row('Last position update', displayRelativeTime(details.node.last_seen_position_at))
      ])
    },
    {
      title: 'Telemetry',
      rows: compactRows([
        row('Voltage', displayValue(details.telemetry?.power?.voltage)),
        row('Battery level', displayValue(details.telemetry?.power?.battery_level)),
        row('Temperature (C)', displayValue(details.telemetry?.environment?.temperature_c)),
        row('Humidity', displayValue(details.telemetry?.environment?.humidity)),
        row('Pressure (hPa)', displayValue(details.telemetry?.environment?.pressure_hpa)),
        row('PM2.5', displayValue(details.telemetry?.air_quality?.pm25)),
        row('PM10', displayValue(details.telemetry?.air_quality?.pm10)),
        row('CO2', displayValue(details.telemetry?.air_quality?.co2)),
        row('IAQ', displayValue(details.telemetry?.air_quality?.iaq))
      ])
    },
    {
      title: 'Source / Timestamps',
      rows: compactRows([
        row('Telemetry source channel', displayValue(details.telemetry?.source_channel)),
        row('Telemetry reported at', displayRelativeTime(details.telemetry?.reported_at)),
        row('Telemetry observed at', displayRelativeTime(details.telemetry?.observed_at)),
        row('Telemetry updated at', displayRelativeTime(details.telemetry?.updated_at))
      ])
    }
  ].filter((section) => section.rows.length > 0)
}

export function NodesPage({ items, selected, details, loadError, onSelect }: Props) {
  const sections = details ? detailSections(details) : []

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
            {sections.map((section) => (
              <section key={section.title}>
                <h4>{section.title}</h4>
                {section.rows.map((item) => (
                  <p key={item.label}>{item.label}: {item.value}</p>
                ))}
              </section>
            ))}
          </>
        ) : <p>Select node</p>}
      </article>
    </section>
  )
}
