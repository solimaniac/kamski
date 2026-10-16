package openrouter

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"kamski/internal/domain"
)

func TestPinnedEndpointMustIndividuallySupportAllParameters(t *testing.T) {
	for _, tt := range []struct {
		name      string
		endpoints string
		available bool
	}{
		{"complete pinned endpoint", `[{"provider_name":"pinned","supported_parameters":["tools","temperature"]}]`, true},
		{"complete unpinned endpoint", `[{"provider_name":"other","supported_parameters":["tools","temperature"]}]`, false},
		{"partial pinned endpoints", `[{"provider_name":"pinned","supported_parameters":["tools"]},{"provider_name":"pinned","supported_parameters":["temperature"]}]`, false},
		{"later complete endpoint", `[{"provider_name":"pinned","supported_parameters":["tools"]},{"provider_name":"pinned","supported_parameters":["tools","temperature"]}]`, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/v1/models/author/model/endpoints" {
					t.Errorf("unexpected path %s", r.URL.Path)
				}
				fmt.Fprintf(w, `{"data":{"endpoints":%s}}`, tt.endpoints)
			}))
			defer server.Close()
			c := client(t, server, 0)
			err := c.checkPinnedEndpoints(context.Background(), "author/model", []string{"pinned"}, []string{"tools", "temperature"})
			if tt.available {
				if err != nil {
					t.Fatal(err)
				}
			} else {
				var failure *domain.Failure
				if !errors.As(err, &failure) || failure.Kind != "unavailable" {
					t.Fatalf("expected unavailable, got %v", err)
				}
			}
		})
	}
}
