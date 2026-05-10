import { beforeEach, describe, expect, it, vi } from 'vitest'

import {
  LeafletMapAdapter,
  OVERLAP_CLUSTER_RADIUS_PX,
  OVERLAP_SPIDERFY_DISTANCE_MULTIPLIER,
  markerClusterOptions
} from './leafletMap'

const leafletMock = vi.hoisted(() => {
  type MarkerHandlerName = 'popupopen' | 'popupclose' | 'mouseover' | 'mouseout'

  class MockLayer {
    public addTo = vi.fn(() => this)
    public clearLayers = vi.fn()
    public removeLayer = vi.fn()
  }

  class MockPopup {
    public content = ''
    public open = false
    public element = {
      addEventListener: vi.fn(),
      removeEventListener: vi.fn()
    }

    public getElement(): typeof this.element {
      return this.element
    }

    public isOpen(): boolean {
      return this.open
    }

    public setContent(content: string): this {
      this.content = content

      return this
    }
  }

  class MockMarker {
    public readonly handlers: Partial<Record<MarkerHandlerName, () => void>> = {}
    public readonly popup = new MockPopup()
    public readonly setIcon = vi.fn(() => this)
    public readonly setLatLng = vi.fn((latLng: unknown) => {
      this.latLng = latLng

      return this
    })
    public readonly openPopup = vi.fn(() => {
      this.popup.open = true
      this.handlers.popupopen?.()

      return this
    })
    public readonly closePopup = vi.fn(() => {
      this.popup.open = false
      this.handlers.popupclose?.()

      return this
    })
    public readonly off = vi.fn()

    public constructor(public latLng: unknown) {}

    public bindPopup(content: string): this {
      this.popup.content = content

      return this
    }

    public on(name: MarkerHandlerName, handler: () => void): this {
      this.handlers[name] = handler

      return this
    }

    public addTo(): this {
      return this
    }

    public getLatLng(): { lat: number; lng: number } {
      const [lat, lng] = this.latLng as [number, number]

      return { lat, lng }
    }

    public getPopup(): MockPopup {
      return this.popup
    }
  }

  class MockMarkerClusterGroup extends MockLayer {
    public readonly options: unknown
    public zoomToShowLayer = vi.fn()

    public constructor(options: unknown) {
      super()
      this.options = options
    }
  }

  class MockMap {
    public setView = vi.fn(() => this)
    public getCenter = vi.fn(() => ({ lat: 0, lng: 0 }))
    public getZoom = vi.fn(() => 10)
    public on = vi.fn()
    public closePopup = vi.fn()
    public panTo = vi.fn()
    public remove = vi.fn()
  }

  const markerClusterGroup = vi.fn((options: unknown) => new MockMarkerClusterGroup(options))
  const marker = vi.fn((latLng: unknown) => new MockMarker(latLng))
  const map = vi.fn(() => new MockMap())
  const tileLayer = vi.fn(() => new MockLayer())
  const featureGroup = vi.fn(() => new MockLayer())
  const icon = vi.fn((options: unknown) => options)
  const circle = vi.fn(() => new MockLayer())

  return {
    circle,
    featureGroup,
    icon,
    map,
    marker,
    markerClusterGroup,
    MockMarkerClusterGroup,
    tileLayer
  }
})

interface PopupOpenMarker {
  handlers: {
    popupopen?: () => void
  }
}

vi.mock('leaflet', () => ({
  default: {
    featureGroup: leafletMock.featureGroup,
    icon: leafletMock.icon,
    map: leafletMock.map,
    marker: leafletMock.marker,
    markerClusterGroup: leafletMock.markerClusterGroup,
    MarkerClusterGroup: leafletMock.MockMarkerClusterGroup,
    circle: leafletMock.circle,
    tileLayer: leafletMock.tileLayer
  }
}))

vi.mock('leaflet.markercluster', () => ({}))

describe('markerClusterOptions', () => {
  it('keeps broad clustering radius behavior when clustering is enabled', () => {
    expect(markerClusterOptions(true)).toEqual({
      chunkedLoading: true,
      removeOutsideVisibleBounds: true,
      showCoverageOnHover: false,
      spiderfyOnMaxZoom: true
    })
  })

  it('uses a small spiderfy-first cluster radius when broad clustering is disabled', () => {
    expect(markerClusterOptions(false)).toEqual({
      chunkedLoading: true,
      maxClusterRadius: OVERLAP_CLUSTER_RADIUS_PX,
      removeOutsideVisibleBounds: true,
      showCoverageOnHover: false,
      spiderfyDistanceMultiplier: OVERLAP_SPIDERFY_DISTANCE_MULTIPLIER,
      spiderfyOnEveryZoom: true,
      spiderfyOnMaxZoom: true,
      zoomToBoundsOnClick: false
    })
  })
})

describe('LeafletMapAdapter', () => {
  beforeEach(() => {
    leafletMock.featureGroup.mockClear()
    leafletMock.icon.mockClear()
    leafletMock.map.mockClear()
    leafletMock.marker.mockClear()
    leafletMock.markerClusterGroup.mockClear()
    leafletMock.tileLayer.mockClear()
  })

  it('creates a markercluster layer when broad clustering is disabled', () => {
    const adapter = new LeafletMapAdapter({} as HTMLElement, [0, 0], 10, {
      clustering: false
    })

    expect(leafletMock.markerClusterGroup).toHaveBeenCalledWith(markerClusterOptions(false))
    adapter.destroy()
  })

  it('creates a markercluster layer with broad clustering options when enabled', () => {
    const adapter = new LeafletMapAdapter({} as HTMLElement, [0, 0], 10, {
      clustering: true
    })

    expect(leafletMock.markerClusterGroup).toHaveBeenCalledWith(markerClusterOptions(true))
    adapter.destroy()
  })

  it('does not recursively render the marker layer when a popup opens', () => {
    const adapter = new LeafletMapAdapter({} as HTMLElement, [0, 0], 10, {
      clustering: false
    })
    adapter.render([
      {
        node: {
          node_id: '!11111111',
          last_seen_any_event_at: '2026-05-10T12:00:00Z'
        },
        position: {
          node_id: '!11111111',
          latitude: 50,
          longitude: 14,
          source_kind: 'position',
          observed_at: '2026-05-10T12:00:00Z'
        }
      }
    ])
    const renderSpy = vi.spyOn(adapter, 'render')
    const marker = leafletMock.marker.mock.results[0]?.value as PopupOpenMarker | undefined

    marker?.handlers.popupopen?.()

    expect(renderSpy).not.toHaveBeenCalled()
    adapter.destroy()
  })
})
