import { Fragment } from 'preact'

import { LogEventList } from '../components/LogEventList'
import { defaultMarkerDataUrl } from '../maps/markerIcons'
import { relativeTime } from '../utils/time'
import { neighborTimeLabel, sortedNeighbors, topologyEvidenceLabel, topologySignalLabel } from '../utils/topology'

import type { LogEvent, NodeDetails, NodeSummary } from '../api/types'
import type { ComponentChildren, JSX } from 'preact'

interface Props {
  items: NodeSummary[]
  selected?: string
  details?: NodeDetails
  filter?: string
  loading?: boolean
  loadError?: string
  recentEvents?: LogEvent[]
  recentEventsLoading?: boolean
  recentEventsError?: string
  onOpenMap: (id: string) => void
  onOpenNodeDetails?: (id: string) => void
  onFilter?: (filter: string) => void
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

const defaultMapMarkerIconURL = defaultMarkerDataUrl()

function displayValue(v: string | number | boolean | undefined): string | null {
  if (typeof v === 'boolean') {return v ? 'yes' : 'no'}
  if (typeof v === 'number') {return String(v)}

  return v && v.length > 0 ? v : null
}

function displayRelativeTime(v?: string): string | null {
  return v ? relativeTime(v) : null
}

function displayNodeLabel(nodeID?: string, displayName?: string): ComponentChildren | null {
  if (!nodeID) {return null}

  return displayName && displayName !== nodeID ? <><span>{displayName}</span> <code>{nodeID}</code></> : <code>{nodeID}</code>
}

function row(label: string, value: ComponentChildren | null): DetailRow | null {
  return value === null ? null : { label, value }
}

function compactRows(rows: (DetailRow | null)[]): DetailRow[] {
  return rows.filter((item): item is DetailRow => item !== null)
}

function previousNameLabel(item: NonNullable<NodeDetails['previous_names']>[number]): string | null {
  const parts = [
    item.previous_long_name ? `Long: ${item.previous_long_name}` : null,
    item.previous_short_name ? `Short: ${item.previous_short_name}` : null
  ].filter((part): part is string => part !== null)

  return parts.length > 0 ? parts.join(' / ') : null
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
        row('Last MQTT via', displayNodeLabel(details.node.last_mqtt_uploader_node_id, details.node.last_mqtt_uploader_display_name)),
        row('Last MQTT via at', displayRelativeTime(details.node.last_mqtt_uploader_at)),
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
        row('Online local nodes', displayValue(details.node.neighbor_nodes_count)),
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
        row('MQTT via', displayNodeLabel(details.position?.mqtt_uploader_node_id, details.position?.mqtt_uploader_display_name)),
        row('Reported at', displayRelativeTime(details.position?.reported_at)),
        row('Observed at', displayRelativeTime(details.position?.observed_at)),
        row('Last position update', displayRelativeTime(details.node.last_seen_position_at))
      ])
    },
    {
      title: 'Telemetry',
      rows: compactRows([
        row('Voltage', displayValue(details.telemetry?.power.voltage)),
        row('Battery level', displayValue(details.telemetry?.power.battery_level)),
        row('Temperature (C)', displayValue(details.telemetry?.environment.temperature_c)),
        row('Humidity', displayValue(details.telemetry?.environment.humidity)),
        row('Pressure (hPa)', displayValue(details.telemetry?.environment.pressure_hpa)),
        row('PM2.5', displayValue(details.telemetry?.air_quality.pm25)),
        row('PM10', displayValue(details.telemetry?.air_quality.pm10)),
        row('CO2', displayValue(details.telemetry?.air_quality.co2)),
        row('IAQ', displayValue(details.telemetry?.air_quality.iaq))
      ])
    },
    {
      title: 'Source / Timestamps',
      rows: compactRows([
        row('Telemetry source channel', displayValue(details.telemetry?.source_channel)),
        row('Telemetry MQTT via', displayNodeLabel(details.telemetry?.mqtt_uploader_node_id, details.telemetry?.mqtt_uploader_display_name)),
        row('Telemetry reported at', displayRelativeTime(details.telemetry?.reported_at)),
        row('Telemetry observed at', displayRelativeTime(details.telemetry?.observed_at)),
        row('Telemetry updated at', displayRelativeTime(details.telemetry?.updated_at))
      ])
    }
  ].filter((section) => section.rows.length > 0)
}

function matchesFilter(item: NodeSummary, rawFilter: string): boolean {
  const filter = rawFilter.trim().toLowerCase()
  if (!filter) {return true}

  return [
    item.node_id,
    item.short_name,
    item.long_name
  ].some((value) => value?.toLowerCase().includes(filter))
}

export function NodesPage({
  items,
  selected,
  details,
  filter = '',
  loading,
  loadError,
  recentEvents = [],
  recentEventsLoading,
  recentEventsError,
  onOpenMap,
  onOpenNodeDetails = () => undefined,
  onFilter = () => undefined,
  onSelect
}: Props): JSX.Element {
  const sections = details ? detailSections(details) : []
  const previousNames = (details?.previous_names ?? [])
    .map((item) => ({ item, label: previousNameLabel(item) }))
    .filter((entry): entry is { item: NonNullable<NodeDetails['previous_names']>[number]; label: string } => entry.label !== null)
  const neighbors = sortedNeighbors(details)
  const filteredItems = items.filter((item) => matchesFilter(item, filter))

  return (
    <section className="nodes-layout container-fluid">
      <article className="node-list-panel">
        <input
          id="nodes-filter"
          type="search"
          className="node-list-filter"
          aria-label="Filter nodes"
          placeholder="Name or ID"
          value={filter}
          onInput={(e) => onFilter((e.currentTarget).value)}
        />
        <div className="node-list" role="list">
          {loadError && <p className="load-error">{loadError}</p>}
          {filteredItems.map((n) => (
            <button
              key={n.node_id}
              type="button"
              className={selected === n.node_id ? 'active' : ''}
              onClick={() => onSelect(n.node_id)}
            >
              <strong>{n.display_name}</strong>
            </button>
          ))}
          {!loadError && filteredItems.length === 0 && <p className="node-list-empty">No matching nodes.</p>}
        </div>
      </article>
      <article className="node-details">
        {details ? (
          <>
            <h3>{details.node.long_name ?? details.node.short_name ?? details.node.node_id}</h3>
            {sections.map((section) => (
              <Fragment key={section.title}>
                <section>
                  {section.title === 'Position' && details.position ? (
                    <div className="node-section-heading">
                      <h4>{section.title}</h4>
                      <button
                        type="button"
                        className="node-section-map-link"
                        aria-label="Open node on map"
                        title="Open node on map"
                        onClick={() => onOpenMap(details.node.node_id)}
                      >
                        <img className="node-section-map-icon" src={defaultMapMarkerIconURL} alt="" aria-hidden="true" />
                      </button>
                    </div>
                  ) : (
                    <h4>{section.title}</h4>
                  )}
                  {section.rows.map((item) => (
                    <p key={item.label}>{item.label}: {item.value}</p>
                  ))}
                </section>
                {section.title === 'Identity' && previousNames.length > 0 && (
                  <section>
                    <h4>Previously known as</h4>
                    {previousNames.map(({ item, label }) => (
                      <p key={`${item.changed_at}:${label}`}>{label} <span className="muted">({relativeTime(item.changed_at)})</span></p>
                    ))}
                  </section>
                )}
              </Fragment>
            ))}
            <section>
              <h4>Neighbors</h4>
              {neighbors.length > 0 ? (
                <div className="node-neighbors-list" role="list">
                  {neighbors.map((neighbor) => (
                    <div className="node-neighbor-card" key={neighbor.node_id} role="listitem">
                      <div className="node-neighbor-head">
                        <strong>{neighbor.display_name}</strong>
                        {neighbor.has_position && (
                          <button
                            type="button"
                            className="node-section-map-link"
                            aria-label={`Open ${neighbor.display_name} on map`}
                            title="Open neighbor on map"
                            onClick={() => onOpenMap(neighbor.node_id)}
                          >
                            <img className="node-section-map-icon" src={defaultMapMarkerIconURL} alt="" aria-hidden="true" />
                          </button>
                        )}
                      </div>
                      <p>ID: <code>{neighbor.node_id}</code></p>
                      <p>Evidence: {topologyEvidenceLabel(neighbor)}</p>
                      <p>Signal: {topologySignalLabel(neighbor)}</p>
                      {neighborTimeLabel(neighbor.last_observed_at) && <p>Last observed: {neighborTimeLabel(neighbor.last_observed_at)}</p>}
                    </div>
                  ))}
                </div>
              ) : (
                <p className="node-list-empty">No topology neighbors available.</p>
              )}
            </section>
            <section className="node-recent-events">
              <h4>Recent events</h4>
              {recentEventsLoading ? (
                <p className="node-list-empty">Loading recent events...</p>
              ) : recentEventsError ? (
                <p className="load-error">{recentEventsError}</p>
              ) : (
                <LogEventList
                  items={recentEvents}
                  showNodeColumn={false}
                  compact
                  maxBodyRows={10}
                  emptyText="No recent events."
                  onOpenNodeDetails={onOpenNodeDetails}
                />
              )}
            </section>
          </>
        ) : <p>{loading ? 'Loading node details...' : 'Select node'}</p>}
      </article>
    </section>
  )
}
