import type { ComponentChildren, VNode } from 'preact'

interface JsonDetailsViewProps {
  value: unknown
}

interface JsonRenderOptions {
  keyPrefix?: string
  trailingComma?: boolean
}

const INDENT_SIZE_REM = 1

function JsonLine({
  depth,
  children,
  trailingComma = false
}: {
  depth: number
  children: ComponentChildren
  trailingComma?: boolean
}) {
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

function renderScalar(value: unknown): VNode {
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

function renderPrefix(keyPrefix?: string): ComponentChildren {
  if (!keyPrefix) return null

  return (
    <>
      <span className="json-key">{JSON.stringify(keyPrefix)}</span>
      <span className="json-punctuation">: </span>
    </>
  )
}

function renderValue(
  value: unknown,
  depth: number,
  options: JsonRenderOptions = {}
): VNode[] {
  const prefix = renderPrefix(options.keyPrefix)

  if (Array.isArray(value)) {
    if (value.length === 0) {
      return [
        <JsonLine key={`array-empty-${depth}-${options.keyPrefix ?? 'root'}`} depth={depth} trailingComma={options.trailingComma}>
          {prefix}
          <span className="json-punctuation">[]</span>
        </JsonLine>
      ]
    }

    const lines: VNode[] = [
      <JsonLine key={`array-open-${depth}-${options.keyPrefix ?? 'root'}`} depth={depth}>
        {prefix}
        <span className="json-punctuation">[</span>
      </JsonLine>
    ]

    value.forEach((item, index) => {
      lines.push(
        ...renderValue(item, depth + 1, {
          trailingComma: index < value.length - 1
        }).map((line, lineIndex) => (
          <div key={`array-item-${depth}-${index}-${lineIndex}`}>{line}</div>
        ))
      )
    })

    lines.push(
      <JsonLine
        key={`array-close-${depth}-${options.keyPrefix ?? 'root'}`}
        depth={depth}
        trailingComma={options.trailingComma}
      >
        <span className="json-punctuation">]</span>
      </JsonLine>
    )

    return lines
  }

  if (value && typeof value === 'object') {
    const entries = Object.entries(value as Record<string, unknown>)
    if (entries.length === 0) {
      return [
        <JsonLine key={`object-empty-${depth}-${options.keyPrefix ?? 'root'}`} depth={depth} trailingComma={options.trailingComma}>
          {prefix}
          <span className="json-punctuation">{'{}'}</span>
        </JsonLine>
      ]
    }

    const lines: VNode[] = [
      <JsonLine key={`object-open-${depth}-${options.keyPrefix ?? 'root'}`} depth={depth}>
        {prefix}
        <span className="json-punctuation">{'{'}</span>
      </JsonLine>
    ]

    entries.forEach(([key, item], index) => {
      lines.push(
        ...renderValue(item, depth + 1, {
          keyPrefix: key,
          trailingComma: index < entries.length - 1
        }).map((line, lineIndex) => (
          <div key={`object-item-${depth}-${key}-${lineIndex}`}>{line}</div>
        ))
      )
    })

    lines.push(
      <JsonLine
        key={`object-close-${depth}-${options.keyPrefix ?? 'root'}`}
        depth={depth}
        trailingComma={options.trailingComma}
      >
        <span className="json-punctuation">{'}'}</span>
      </JsonLine>
    )

    return lines
  }

  return [
    <JsonLine key={`scalar-${depth}-${options.keyPrefix ?? 'root'}`} depth={depth} trailingComma={options.trailingComma}>
      {prefix}
      {renderScalar(value)}
    </JsonLine>
  ]
}

export function JsonDetailsView({ value }: JsonDetailsViewProps) {
  return (
    <div className="json-view">
      {renderValue(value, 0)}
    </div>
  )
}
