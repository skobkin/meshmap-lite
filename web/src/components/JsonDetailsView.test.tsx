// @vitest-environment jsdom

import { fireEvent, render, screen, within } from '@testing-library/preact'
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

  it('collapses and expands object nodes without persisting state', () => {
    render(
      <JsonDetailsView
        value={{
          nested: {
            value: 'alpha',
            count: 2
          }
        }}
      />
    )

    fireEvent.click(screen.getByLabelText('Collapse object at $.nested'))

    expect(screen.getByText('{...} 2 keys').classList.contains('json-collapsed')).toBe(true)
    expect(screen.queryByText('"alpha"')).toBeNull()

    fireEvent.click(screen.getByLabelText('Expand object at $.nested'))

    expect(screen.getByText('"alpha"').classList.contains('json-string')).toBe(true)
    expect(screen.getByText('2').classList.contains('json-number')).toBe(true)
  })

  it('collapses array nodes into a one-line summary', () => {
    render(
      <JsonDetailsView
        value={{
          items: ['x', 'y', 'z']
        }}
      />
    )

    fireEvent.click(screen.getByLabelText('Collapse array at $.items'))

    expect(screen.getByText('[...] 3 items').classList.contains('json-collapsed')).toBe(true)
    expect(screen.queryByText('"x"')).toBeNull()
    expect(screen.queryByText('"y"')).toBeNull()
    expect(screen.queryByText('"z"')).toBeNull()
  })
})
