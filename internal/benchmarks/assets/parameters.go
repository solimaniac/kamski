package assets

import (
	"bytes"
	"encoding/json"
	"fmt"

	"kamski/internal/domain"
)

// Parameters validates family-owned typed metadata at asset-load time, while the
// shared result contract remains independent of family-specific Go structs.
func Parameters(v domain.Variant, out any) error {
	b, err := json.Marshal(v.Parameters)
	if err != nil {
		return err
	}
	if err = domain.Decode(bytes.NewReader(b), out); err != nil {
		return fmt.Errorf("variant %s parameters: %w", v.ID, err)
	}
	return nil
}
