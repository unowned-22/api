package handler

import "encoding/json"

// jsonUnmarshalSafe is a thin wrapper around json.Unmarshal used when
// decoding raw JSONB columns (e.g. feed_items.media) into typed DTOs.
// Kept as a tiny named helper so call sites read clearly and so a single
// place can be hardened later (e.g. size limits) if needed.
func jsonUnmarshalSafe(data []byte, v any) error {
	return json.Unmarshal(data, v)
}
