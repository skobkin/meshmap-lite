// Package dedup provides a bounded in-memory TTL window for duplicate suppression.
//
// Entries are stored in recency order. A successful duplicate lookup refreshes the
// entry to the front of the window. Entries expire when their age exceeds the
// configured TTL. When TTL is zero or negative, entries never expire by age and
// are evicted only by the size bound. Size values below 1 are normalized to 1.
package dedup
