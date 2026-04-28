package output

import (
	"encoding/json"
	"fmt"
	"io"
)

type Options struct {
	Pretty bool
	Raw    bool
}

type Envelope struct {
	Query any `json:"query,omitempty"`
	Meta  any `json:"meta,omitempty"`
	Data  any `json:"data"`
}

func WriteJSON(w io.Writer, value any, opts Options) error {
	enc := json.NewEncoder(w)
	if opts.Pretty {
		enc.SetIndent("", "  ")
	}
	return enc.Encode(value)
}

func WriteEnvelope(w io.Writer, query any, meta any, data any, opts Options) error {
	if opts.Raw {
		return WriteJSON(w, data, opts)
	}
	return WriteJSON(w, Envelope{
		Query: query,
		Meta:  meta,
		Data:  data,
	}, opts)
}

func Errorf(format string, args ...any) error {
	return fmt.Errorf(format, args...)
}
