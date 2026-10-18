package domain

import (
	"encoding/json"
	"testing"
)

func TestVariantOptionalLegacyMetadata(t *testing.T) {
	for _, raw := range []string{
		`{"id":"example","title":"Example","description":"Example scenario"}`,
		`{"id":"example","title":"Example","description":"Example scenario","parameters":{}}`,
		`{"id":"example","version":"1.0.0","title":"Example","description":"Example scenario","changes":[],"parameters":{}}`,
		`{"id":"example","version":"1.1.0","title":"Example","description":"Example scenario","changes":["Historical note"],"parameters":{}}`,
	} {
		var v Variant
		if err := json.Unmarshal([]byte(raw), &v); err != nil {
			t.Fatal(err)
		}
		got, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != raw {
			t.Fatalf("variant representation changed (and would alter saved hashes):\ngot  %s\nwant %s", got, raw)
		}
	}
}
