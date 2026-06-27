import type { JSX } from 'preact'

// Tagged result so renderers can distinguish "show this string" from
// "wrap this boolean emoji with a11y attributes" without re-inspecting
// the raw value.
export type RenderedScalar =
  | string
  | { kind: 'boolean'; emoji: string; label: string }

export function formatScalar(value: unknown): RenderedScalar | undefined {
  if (typeof value === 'string') {
    const trimmed = value.trim()

    return trimmed === '' ? undefined : trimmed
  }
  if (typeof value === 'number') {
    return String(value)
  }
  if (typeof value === 'boolean') {
    return {
      kind: 'boolean',
      emoji: value ? '✅' : '❌',
      label: value ? 'yes' : 'no'
    }
  }

  return undefined
}

export function renderScalar(value: RenderedScalar | undefined): JSX.Element | string | null {
  if (value === undefined) {
    return null
  }
  if (typeof value === 'string') {
    return value
  }

  return <span aria-label={value.label} role="img" title={value.label}>{value.emoji}</span>
}
