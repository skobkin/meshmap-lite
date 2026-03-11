import { useState } from 'preact/hooks'

import type { ComponentChildren } from 'preact'

interface JsonDetailsViewProps {
  value: unknown
}

interface JsonNodeProps {
  depth: number
  path: string
  value: unknown
  keyPrefix?: string
  trailingComma?: boolean
}

interface JsonCollectionNodeProps {
  depth: number
  path: string
  keyPrefix?: string
  trailingComma?: boolean
}

const INDENT_SIZE_REM = 1

const JsonLine = ({
  depth,
  children,
  trailingComma = false
}: {
  depth: number
  children: ComponentChildren
  trailingComma?: boolean
}) => {
  return (
    <div className="json-line">
      <span
        aria-hidden="true"
        className="json-indent"
        style={{ width: `${depth * INDENT_SIZE_REM}rem` }}
      />
      {children}
      {trailingComma ? <span className="json-punctuation">,</span> : null}
    </div>
  )
}

function renderScalar(value: unknown) {
  if (typeof value === 'string') {
    return <span className="json-string">{JSON.stringify(value)}</span>
  }

  if (typeof value === 'number' || typeof value === 'bigint') {
    return <span className="json-number">{String(value)}</span>
  }

  if (typeof value === 'boolean') {
    return <span className="json-boolean">{String(value)}</span>
  }

  if (value === null) {
    return <span className="json-null">null</span>
  }

  const fallback = JSON.stringify(value) ?? JSON.stringify(Object.prototype.toString.call(value))
  return <span className="json-string">{fallback}</span>
}

function renderPrefix(keyPrefix?: string) {
  if (!keyPrefix) return null

  return (
    <>
      <span className="json-key">{JSON.stringify(keyPrefix)}</span>
      <span className="json-punctuation">: </span>
    </>
  )
}

function objectSummary(entryCount: number): string {
  return entryCount === 1 ? '{...} 1 key' : `{...} ${entryCount} keys`
}

function arraySummary(itemCount: number): string {
  return itemCount === 1 ? '[...] 1 item' : `[...] ${itemCount} items`
}

const JsonToggle = ({
  expanded,
  onToggle,
  label
}: {
  expanded: boolean
  onToggle: () => void
  label: string
}) => {
  return (
    <button
      aria-label={label}
      className="json-toggle"
      type="button"
      onClick={onToggle}
    >
      <span aria-hidden="true" className="json-toggle-icon">
        {expanded ? '-' : '+'}
      </span>
    </button>
  )
}

const JsonNode = ({ depth, path, value, keyPrefix, trailingComma = false }: JsonNodeProps) => {
  if (Array.isArray(value)) {
    return (
      <JsonArrayNode
        depth={depth}
        keyPrefix={keyPrefix}
        path={path}
        trailingComma={trailingComma}
        value={value}
      />
    )
  }

  if (value && typeof value === 'object') {
    return (
      <JsonObjectNode
        depth={depth}
        keyPrefix={keyPrefix}
        path={path}
        trailingComma={trailingComma}
        value={value as Record<string, unknown>}
      />
    )
  }

  return (
    <JsonLine depth={depth} trailingComma={trailingComma}>
      {renderPrefix(keyPrefix)}
      {renderScalar(value)}
    </JsonLine>
  )
}

const JsonArrayNode = ({
  depth,
  path,
  value,
  keyPrefix,
  trailingComma = false
}: JsonCollectionNodeProps & { value: unknown[] }) => {
  const [expanded, setExpanded] = useState(true)
  const prefix = renderPrefix(keyPrefix)

  if (value.length === 0) {
    return (
      <JsonLine depth={depth} trailingComma={trailingComma}>
        {prefix}
        <span className="json-punctuation">[]</span>
      </JsonLine>
    )
  }

  if (!expanded) {
    return (
      <JsonLine depth={depth} trailingComma={trailingComma}>
        <JsonToggle
          expanded={false}
          label={`Expand array at ${path}`}
          onToggle={() => setExpanded(true)}
        />
        {prefix}
        <span className="json-collapsed">{arraySummary(value.length)}</span>
      </JsonLine>
    )
  }

  return (
    <>
      <JsonLine depth={depth}>
        <JsonToggle
          expanded
          label={`Collapse array at ${path}`}
          onToggle={() => setExpanded(false)}
        />
        {prefix}
        <span className="json-punctuation">[</span>
      </JsonLine>
      {value.map((item, index) => (
        <JsonNode
          depth={depth + 1}
          key={`${path}[${index}]`}
          path={`${path}[${index}]`}
          trailingComma={index < value.length - 1}
          value={item}
        />
      ))}
      <JsonLine depth={depth} trailingComma={trailingComma}>
        <span className="json-toggle-spacer" aria-hidden="true" />
        <span className="json-punctuation">]</span>
      </JsonLine>
    </>
  )
}

const JsonObjectNode = ({
  depth,
  path,
  value,
  keyPrefix,
  trailingComma = false
}: JsonCollectionNodeProps & { value: Record<string, unknown> }) => {
  const [expanded, setExpanded] = useState(true)
  const prefix = renderPrefix(keyPrefix)
  const entries = Object.entries(value)

  if (entries.length === 0) {
    return (
      <JsonLine depth={depth} trailingComma={trailingComma}>
        {prefix}
        <span className="json-punctuation">{'{}'}</span>
      </JsonLine>
    )
  }

  if (!expanded) {
    return (
      <JsonLine depth={depth} trailingComma={trailingComma}>
        <JsonToggle
          expanded={false}
          label={`Expand object at ${path}`}
          onToggle={() => setExpanded(true)}
        />
        {prefix}
        <span className="json-collapsed">{objectSummary(entries.length)}</span>
      </JsonLine>
    )
  }

  return (
    <>
      <JsonLine depth={depth}>
        <JsonToggle
          expanded
          label={`Collapse object at ${path}`}
          onToggle={() => setExpanded(false)}
        />
        {prefix}
        <span className="json-punctuation">{'{'}</span>
      </JsonLine>
      {entries.map(([entryKey, item], index) => (
        <JsonNode
          depth={depth + 1}
          key={`${path}.${entryKey}`}
          keyPrefix={entryKey}
          path={`${path}.${entryKey}`}
          trailingComma={index < entries.length - 1}
          value={item}
        />
      ))}
      <JsonLine depth={depth} trailingComma={trailingComma}>
        <span className="json-toggle-spacer" aria-hidden="true" />
        <span className="json-punctuation">{'}'}</span>
      </JsonLine>
    </>
  )
}

export function JsonDetailsView({ value }: JsonDetailsViewProps) {
  return (
    <div className="json-view">
      <JsonNode depth={0} path="$" value={value} />
    </div>
  )
}
