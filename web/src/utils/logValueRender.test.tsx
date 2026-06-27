// @vitest-environment jsdom

import { render, screen } from '@testing-library/preact'
import { describe, expect, it } from 'vitest'

import { formatScalar, renderScalar } from './logValueRender'

describe('log value rendering', () => {
  it('keeps true booleans accessible while rendering the compact icon', () => {
    render(<>{renderScalar(formatScalar(true))}</>)

    const icon = screen.getByRole('img', { name: 'yes' })

    expect(icon.textContent).toBe('✅')
  })

  it('keeps false booleans accessible while rendering the compact icon', () => {
    render(<>{renderScalar(formatScalar(false))}</>)

    const icon = screen.getByRole('img', { name: 'no' })

    expect(icon.textContent).toBe('❌')
  })
})
