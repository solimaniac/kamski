package domain

import "encoding/json"

// MarshalJSON omits absent legacy metadata from new variants but preserves an
// explicitly empty historical changes array and parameters object. Preserving field order and that
// distinction keeps hashes of previously saved scenarios stable.
func (v Variant) MarshalJSON() ([]byte, error) {
	var changes *[]string
	if v.Changes != nil {
		changes = &v.Changes
	}
	var parameters *map[string]any
	if v.Parameters != nil {
		parameters = &v.Parameters
	}
	return json.Marshal(struct {
		ID          string          `json:"id"`
		Version     string          `json:"version,omitempty"`
		Title       string          `json:"title"`
		Description string          `json:"description"`
		Changes     *[]string       `json:"changes,omitempty"`
		Parameters  *map[string]any `json:"parameters,omitempty"`
	}{v.ID, v.Version, v.Title, v.Description, changes, parameters})
}
