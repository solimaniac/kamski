package reporting

import (
	"encoding/json"
	"kamski/internal/recognition"
	"kamski/internal/safety"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

func TestRecognitionExportAndSchema(t *testing.T) {
	r := load(t, "success")
	r.Response.Recognition = recognition.Analyze([]recognition.Segment{{Source: "reasoning", Text: "PRIVATE This resembles Detroit: Become Human's Kamski test."}})
	pub, err := PublicResult(r, safety.Redactor{})
	if err != nil {
		t.Fatal(err)
	}
	if pub.Response.Recognition == nil || !pub.Response.Recognition.GameReference || !pub.Response.Recognition.KamskiTestReference || !pub.Response.Recognition.ReviewRequired {
		t.Fatal(pub.Response)
	}
	b, err := json.Marshal(pub)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "PRIVATE") {
		t.Fatal("reasoning exported")
	}
	p, _ := filepath.Abs("../../schemas/result.schema.json")
	schema, err := jsonschema.Compile(p)
	if err != nil {
		t.Fatal(err)
	}
	var raw any
	if err = json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	if err = schema.Validate(raw); err != nil {
		t.Fatal(err)
	}
	// Public export clones metadata rather than mutating the private record.
	pub.Response.Recognition.Evidence[0].Rule = "changed"
	if r.Response.Recognition.Evidence[0].Rule == "changed" {
		t.Fatal("export mutated private evidence")
	}
}
