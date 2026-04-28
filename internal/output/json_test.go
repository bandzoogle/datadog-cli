package output

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestWriteEnvelopeWrapsData(t *testing.T) {
	var buf bytes.Buffer
	err := WriteEnvelope(&buf, map[string]any{"command": "logs search"}, map[string]any{"site": "datadoghq.com"}, []string{"one"}, Options{})
	if err != nil {
		t.Fatalf("write envelope: %v", err)
	}

	var got Envelope
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if got.Data == nil {
		t.Fatal("expected data in envelope")
	}
}

func TestWriteEnvelopeRawSkipsEnvelope(t *testing.T) {
	var buf bytes.Buffer
	err := WriteEnvelope(&buf, map[string]any{"command": "logs search"}, nil, map[string]string{"ok": "true"}, Options{Raw: true})
	if err != nil {
		t.Fatalf("write raw: %v", err)
	}
	if bytes.Contains(buf.Bytes(), []byte("query")) {
		t.Fatalf("raw output should not include envelope: %s", buf.String())
	}
}
