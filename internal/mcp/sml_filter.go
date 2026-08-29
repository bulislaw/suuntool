package mcp

import (
	"encoding/json"

	"github.com/bulislaw/suuntool/internal/api"
)

// filterSML parses a raw /v1/workouts/{key}/sml JSON body, keeps only the
// requested inner-Sample fields, optionally drops every-Nth-or-other sample,
// and returns a compact map ready for MCP structured output.
//
// SML shape (see handoff/SUUNTO_API.md): each entry in Data.Samples[] is
// {"TimeISO8601": "...", "Source": "...", "Attributes": {"suunto/sml":
// {"Sample": {...inner fields...}}}}. The inner Sample is what carries the
// useful channels (HR, Power, Cadence, GPSAltitude, Latitude, Longitude,
// EHPE, NumberOfSatellites, UTC, Events, …).
//
// streams: keep only inner fields whose name is in this set; samples that
// have none of the requested fields are dropped entirely. Empty = keep every
// inner field on every sample (still drops the Attributes wrapper).
//
// downsample: keep every Nth sample of the post-filter list. <=1 = keep all.
//
// includeSummary: if true, the parsed Summary block is included verbatim.
func filterSML(body []byte, streams []string, downsample int, includeSummary bool) (map[string]any, error) {
	var doc struct {
		Data struct {
			Samples []map[string]json.RawMessage `json:"Samples"`
		} `json:"Data"`
		Summary json.RawMessage `json:"Summary"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, &api.Error{Code: "SERVER", Message: "decode SML JSON: " + err.Error(), Exit: 5}
	}

	wanted := map[string]struct{}{}
	for _, s := range streams {
		if s != "" {
			wanted[s] = struct{}{}
		}
	}
	filterFields := len(wanted) > 0

	kept := make([]map[string]any, 0, len(doc.Data.Samples))
	for _, raw := range doc.Data.Samples {
		inner := extractInnerSample(raw)
		if inner == nil {
			continue
		}
		if filterFields {
			pruned := map[string]any{}
			for k, v := range inner {
				if _, ok := wanted[k]; ok {
					pruned[k] = v
				}
			}
			if len(pruned) == 0 {
				continue
			}
			inner = pruned
		}
		if ts, ok := raw["TimeISO8601"]; ok {
			var s string
			if json.Unmarshal(ts, &s) == nil && s != "" {
				inner["TimeISO8601"] = s
			}
		}
		kept = append(kept, inner)
	}

	if downsample > 1 && len(kept) > 0 {
		out := make([]map[string]any, 0, len(kept)/downsample+1)
		for i, s := range kept {
			if i%downsample == 0 {
				out = append(out, s)
			}
		}
		kept = out
	}

	res := map[string]any{
		"sample_count": len(kept),
		"samples":      kept,
	}
	if includeSummary && len(doc.Summary) > 0 {
		var sm any
		if err := json.Unmarshal(doc.Summary, &sm); err == nil {
			res["summary"] = sm
		}
	}
	return res, nil
}

// extractInnerSample digs the {"Attributes":{"suunto/sml":{"Sample":{...}}}}
// payload out of a Data.Samples[] entry. Returns nil when the entry has no
// inner Sample (e.g. metadata-only rows).
func extractInnerSample(entry map[string]json.RawMessage) map[string]any {
	attrsRaw, ok := entry["Attributes"]
	if !ok {
		return nil
	}
	var attrs map[string]json.RawMessage
	if json.Unmarshal(attrsRaw, &attrs) != nil {
		return nil
	}
	smlRaw, ok := attrs["suunto/sml"]
	if !ok {
		return nil
	}
	var sml map[string]json.RawMessage
	if json.Unmarshal(smlRaw, &sml) != nil {
		return nil
	}
	sampleRaw, ok := sml["Sample"]
	if !ok {
		return nil
	}
	var sample map[string]any
	if json.Unmarshal(sampleRaw, &sample) != nil {
		return nil
	}
	return sample
}
