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
 * Classify the number of hops a packet traversed before reaching the MQTT
 * gateway, based on the Meshtastic `hop_start` / `hop_limit` header values.
 *
 *  - 1-2 hops:                 good (close to the gateway, strong link)
 *  - 3 hops up to (hop_start-2): warn (mid-distance, expected relay behavior)
 *  - last 2 hops of the budget:  bad  (one more hop and the packet would die)
 *  - hop_limit=0 with hop_start>0: "exhausted" — the packet was one hop from
 *    being dropped; always rendered with the "bad" colour but with a distinct
 *    CSS modifier so it can be styled differently in the future.
 *  - hop_start=0, hop_limit=0, or hop_start<=hop_limit: unknown / self-upload
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

  // Self-uploaded packet (no hops consumed between origin and gateway).
  if (hopStart <= hopLimit) {
    return { qualityClass: '', title: 'Hops traversed: 0 (self-uploaded)', exhausted: false }
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
