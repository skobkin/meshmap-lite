// @vitest-environment jsdom

import { render, screen, within } from '@testing-library/preact'
import { describe, expect, it } from 'vitest'

import { JsonDetailsView } from './JsonDetailsView'

describe('JsonDetailsView', () => {
  it('renders typed tokens for nested JSON values', () => {
    render(
      <JsonDetailsView
        value={{
          name: 'alpha',
          count: 2,
          active: true,
          missing: null,
          nested: {
            items: ['x']
          }
        }}
      />
    )

    expect(screen.getByText('"name"').classList.contains('json-key')).toBe(true)
    expect(screen.getByText('"alpha"').classList.contains('json-string')).toBe(true)
    expect(screen.getByText('2').classList.contains('json-number')).toBe(true)
    expect(screen.getByText('true').classList.contains('json-boolean')).toBe(true)
    expect(screen.getByText('null').classList.contains('json-null')).toBe(true)
    expect(screen.getByText('"items"').classList.contains('json-key')).toBe(true)
    expect(screen.getByText('[').classList.contains('json-punctuation')).toBe(true)
    expect(screen.getByText(']').classList.contains('json-punctuation')).toBe(true)
  })

  it('renders empty objects and arrays compactly', () => {
    const { container } = render(
      <>
        <JsonDetailsView value={{}} />
        <JsonDetailsView value={[]} />
      </>
    )

    const views = container.querySelectorAll('.json-view')
    expect(within(views[0] as HTMLElement).getByText('{}')).toBeTruthy()
    expect(within(views[1] as HTMLElement).getByText('[]')).toBeTruthy()
  })

  it('uses JSON escaping rules for strings', () => {
    render(<JsonDetailsView value={{ note: 'line\n"quote"' }} />)

    expect(screen.getByText('"line\\n\\"quote\\""').classList.contains('json-string')).toBe(true)
  })
})
