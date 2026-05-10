package domain

// IsValidPosition reports whether coordinates should be treated as a real
// position. Meshtastic devices sometimes emit the null-island pair when no
// usable fix is available.
func IsValidPosition(latitude, longitude float64) bool {
	return latitude != 0 || longitude != 0
}
