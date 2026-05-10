import { describe, expect, it } from 'vitest'

import { parseFragmentState, serializeFragmentState } from './urlState'

describe('urlState', () => {
  it('parses map route fragments with encoded node, chat, and panel state', () => {
    expect(parseFragmentState('#/map?lat=64.5&lng=40.6&z=12&node=%21abc&chat=mesh%20room&chat_panel=open&ignored=1')).toEqual({
      page: 'map',
      map: {
        view: { center: [64.5, 40.6], zoom: 12 },
        node: '!abc',
        chatChannel: 'mesh room',
        chatPanel: 'open'
      }
    })
  })

  it('drops invalid map params without failing', () => {
    expect(parseFragmentState('#/map?lat=wat&lng=200&z=25&node=&chat_panel=nope')).toEqual({
      page: 'map',
      map: {
        view: undefined,
        node: undefined,
        chatChannel: undefined,
        chatPanel: undefined
      }
    })
  })

  it('does not treat the old map-only hash format as state', () => {
    expect(parseFragmentState('#lat=64.5&lng=40.6&z=12')).toEqual({ page: 'map' })
  })

  it('parses nodes state', () => {
    expect(parseFragmentState('#/nodes?node=%21abc&q=relay%20one')).toEqual({
      page: 'nodes',
      nodes: {
        node: '!abc',
        q: 'relay one'
      }
    })
  })

  it('parses repeated log event kinds and ignores invalid values', () => {
    expect(parseFragmentState('#/log?event_kind=7&event_kind=bad&event_kind=4&event_kind=-1&channel=mesh&node_id=%21abc')).toEqual({
      page: 'log',
      log: {
        eventKinds: [7, 4],
        channel: 'mesh',
        nodeID: '!abc'
      }
    })
  })

  it('serializes canonical route fragments', () => {
    expect(serializeFragmentState({
      page: 'log',
      log: {
        eventKinds: [7, 4],
        channel: 'mesh room',
        nodeID: '!abc'
      }
    })).toBe('#/log?event_kind=7&event_kind=4&channel=mesh+room&node_id=%21abc')
  })
})
