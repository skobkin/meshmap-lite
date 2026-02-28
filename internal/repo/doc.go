// Package repo defines repository-facing DTOs and narrow read/write ports used
// by higher-level packages.
//
// Write-side ports are consumed by ingest. Read-side ports are consumed by HTTP
// and other query-serving code. Query and view DTOs that describe repository
// operations live here so transport and storage packages do not need to invent
// their own copies.
package repo
