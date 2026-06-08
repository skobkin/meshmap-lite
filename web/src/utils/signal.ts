// Shared helpers for rendering a number of "hops traversed" with a quality
// classification that maps to the universal `.signal-good` / `.signal-warn` /
// `.signal-bad` CSS classes (reusable for RSSI / SNR etc. later without
// renaming).

export type SignalQuality = 'good' | 'warn' | 'bad' | 'exhausted' | 'none'

export interface HopsClassification {
  /** Number of hops the packet actually traversed, or undefined when unknown. */
  traversed?: number
  /** CSS modifier class to apply alongside the badge. */
  qualityClass: string
  /** Human-readable title for the badge tooltip. */
  title: string
  /** True when the packet used up the last of its hop budget. */
  exhausted: boolean
}

/**
 * Classify the number of LoRa rebroadcasts a packet went through before
 * reaching the MQTT gateway, based on the Meshtastic `hop_start` / `hop_limit`
 * header values. Note that a "hop" in Meshtastic counts *rebroadcasts* (relay
 * hops) — a single direct LoRa transmission between two radios is zero hops,
 * and the first relay's rebroadcast is hop 1.
 *
 * The classifier always returns a non-negative `traversed` value (0..N) when
 * the hop header is present and well-formed. The renderer decides whether to
 * show a `↓0` badge or hide it; today the chat panel hides it and the log
 * page shows it.
 *
 *  - 1-2 hops:                 good (well within the configured budget)
 *  - 3 hops up to (hop_start-2): warn (mid-distance, expected relay behaviour)
 *  - last 2 hops of the budget:  bad  (one more hop and the packet would die)
 *  - hop_limit=0 with hop_start>0: "exhausted" — the packet used the very last
 *    of its hop budget; always rendered with the "bad" colour but with a
 *    distinct CSS modifier so it can be styled differently in the future.
 *  - hop_start=0, hop_limit=0, or hop_start<=hop_limit: direct transmission
 *    to the uploader (no intermediate LoRa rebroadcast). `traversed` is 0
 *    with no quality-class modifier; whether the renderer shows a `↓0` badge
 *    is a per-view decision.
 *  - any other invalid combination (negative values, non-number): no badge
 *    at all (the classifier returns `traversed: undefined`).
 */
export function classifyHops(hopStart: number | undefined, hopLimit: number | undefined): HopsClassification {
  if (
    typeof hopStart !== 'number' ||
    typeof hopLimit !== 'number' ||
    hopStart <= 0 ||
    hopLimit < 0
  ) {
    return { qualityClass: '', title: '', exhausted: false }
  }

  // Direct transmission to the uploader with no intermediate LoRa
  // rebroadcast (traversed === 0). This is most often a direct LoRa
  // transmission to a node that is itself an MQTT gateway, but it also
  // covers the case where the originator is the uploader. The two are
  // observationally indistinguishable from the hop-header values alone.
  //
  // We return `traversed: 0` here (with a neutral quality class) so the
  // log view can show a `↓0` badge for consistency with the `↓N` badges
  // it already renders. The chat view still hides `↓0` to keep the
  // per-line visual weight down — that hiding is done by the renderer,
  // not by the classifier.
  if (hopStart <= hopLimit) {
    return { traversed: 0, qualityClass: '', title: 'Hops traversed: 0 (direct transmission to uploader)', exhausted: false }
  }

  const traversed = hopStart - hopLimit

  // Packet used the very last of its hop budget.
  if (hopLimit === 0) {
    return {
      traversed,
      qualityClass: 'signal-bad signal-exhausted',
      title: `Hops traversed: ${traversed} (hop budget exhausted)`,
      exhausted: true,
    }
  }

  let quality: SignalQuality = 'warn'
  if (traversed <= 2) {
    quality = 'good'
  } else if (hopLimit <= 1) {
    // Within the last hop of the configured budget — the packet is one
    // relay away from being dropped on the next transmission.
    quality = 'bad'
  }

  return {
    traversed,
    qualityClass: `signal-${quality}`,
    title: `Hops traversed: ${traversed}`,
    exhausted: false,
  }
}
