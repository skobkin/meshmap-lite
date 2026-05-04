import { beforeEach, describe, expect, it, vi } from 'vitest'

import { api } from './client'

describe('api client', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  it('builds log event query strings from optional filters', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => []
    })
    vi.stubGlobal('fetch', fetchMock)

    await api.logEvents({
      limit: 50,
      before: 42,
      eventKinds: [1, 9],
      channel: 'ops room'
    })

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/log/events?limit=50&before=42&channel=ops+room&event_kind=1&event_kind=9',
      { signal: undefined }
    )
  })

  it('encodes node ids in detail requests', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({})
    })
    vi.stubGlobal('fetch', fetchMock)

    await api.node('!ab/cd')

    expect(fetchMock).toHaveBeenCalledWith('/api/v1/nodes/!ab%2Fcd', { signal: undefined })
  })

  it('requests activity stats', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ periods: [] })
    })
    vi.stubGlobal('fetch', fetchMock)

    await api.statsActivity()

    expect(fetchMock).toHaveBeenCalledWith('/api/v1/stats/activity', { signal: undefined })
  })

  it('builds topology edge query strings from optional filters', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => []
    })
    vi.stubGlobal('fetch', fetchMock)

    await api.topologyEdges({
      nodeID: '!ab/cd',
      channel: 'ops room',
      sourceKinds: ['neighbor_info', 'routing_return']
    })

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/topology/edges?node_id=%21ab%2Fcd&channel=ops+room&source_kind=neighbor_info&source_kind=routing_return',
      { signal: undefined }
    )
  })

  it('throws a status-based error when a request fails', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: false,
      status: 503
    }))

    await expect(api.meta()).rejects.toThrow('request failed: 503')
  })
})
