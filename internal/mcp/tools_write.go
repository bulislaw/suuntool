package mcp

import (
	"bytes"
	"context"
	"encoding/base64"
	"os"
	"strings"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bulislaw/suuntool/internal/api"
	"github.com/bulislaw/suuntool/internal/api/endpoints"
	"github.com/bulislaw/suuntool/internal/session"
)

// validExtensionTypes is the membership set of endpoints.DefaultExtensionTypes,
// rebuilt once for O(1) lookups in the workouts_extensions registrar. Passing
// an unknown extension type to /v1/workout/extensions/{key} returns an opaque
// HTTP 500 from Suunto, so we reject upfront with a USAGE error.
var validExtensionTypes = func() map[string]struct{} {
	m := make(map[string]struct{}, len(endpoints.DefaultExtensionTypes))
	for _, t := range endpoints.DefaultExtensionTypes {
		m[t] = struct{}{}
	}
	return m
}()

// --- write-tier arg structs ---

type workoutsCommentArgs struct {
	Key  string `json:"key" jsonschema:"the workout key to comment on"`
	Text string `json:"text" jsonschema:"comment body (plain text)"`
}

type workoutsReactArgs struct {
	Key string `json:"key" jsonschema:"the workout key to react on"`
}

type workoutsEditArgs struct {
	Key   string         `json:"key" jsonschema:"workout key to update"`
	Patch map[string]any `json:"patch" jsonschema:"attribute changes to apply (server-shaped JSON object)"`
}

type workoutsBatchUpdateArgs struct {
	Entries []map[string]any `json:"entries" jsonschema:"array of partial-update entries (each must include a 'key' field)"`
}

type workoutsShareArgs struct {
	Username string `json:"username,omitempty" jsonschema:"username that owns the workout; empty = authenticated user"`
	Key      string `json:"key" jsonschema:"workout key to share"`
	Format   string `json:"format,omitempty" jsonschema:"share format; one of gpx-route (default) or gpx-track"`
}

type workoutsExtensionsArgs struct {
	Key   string   `json:"key" jsonschema:"workout key"`
	Types []string `json:"types,omitempty" jsonschema:"extension types to request; empty = default Android set. Valid values: SummaryExtension, FitnessExtension, SkiExtension, IntensityExtension, DiveHeaderExtension, SwimmingHeaderExtension, WeatherExtension, JumpRopeExtension."`
}

type workoutsUploadArgs struct {
	SMLBase64        string `json:"sml_base64" jsonschema:"base64-encoded .sml file body"`
	ExtensionsBase64 string `json:"extensions_base64,omitempty" jsonschema:"optional base64-encoded extensions.json"`
}

type guidesUploadArgs struct {
	ZipBase64 string `json:"zip_base64" jsonschema:"base64-encoded guide zip archive (manifest.json + guide.json + icon.png)"`
}

type guidesUpdateArgs struct {
	ID        string `json:"id" jsonschema:"guide id to update"`
	ZipBase64 string `json:"zip_base64" jsonschema:"base64-encoded guide zip archive to replace the guide's content with"`
}

type guidesPinArgs struct {
	ID string `json:"id" jsonschema:"guide id to pin"`
}

type guidesUnpinArgs struct {
	ID string `json:"id" jsonschema:"guide id to unpin"`
}

// writeRegistrars returns the tierWrite tool registrars.
func writeRegistrars() []toolRegistrar {
	return []toolRegistrar{
		// workouts_comment
		func(s *sdkmcp.Server, d *deps) {
			sdkmcp.AddTool(s, &sdkmcp.Tool{
				Name:        "workouts_comment",
				Description: "Post a comment on a workout (POST /v1/workouts/comment/{key}). Requires --allow-write.",
			}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, a workoutsCommentArgs) (*sdkmcp.CallToolResult, any, error) {
				if e := authGate(d); e != nil {
					return e, nil, nil
				}
				v, err := endpoints.PostComment(ctx, d.client, a.Key, a.Text, session.TOTPHeaders(d.session))
				if err != nil {
					return mapErrorToCallToolResult(err), nil, nil
				}
				return nil, map[string]any{"key": a.Key, "payload": v}, nil
			})
		},

		// workouts_react
		func(s *sdkmcp.Server, d *deps) {
			sdkmcp.AddTool(s, &sdkmcp.Tool{
				Name:        "workouts_react",
				Description: "Add a 'like' reaction to a workout (POST /v1/workouts/reaction/{key}). Requires --allow-write.",
			}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, a workoutsReactArgs) (*sdkmcp.CallToolResult, any, error) {
				if e := authGate(d); e != nil {
					return e, nil, nil
				}
				v, err := endpoints.AddReaction(ctx, d.client, a.Key, endpoints.ReactionLike, session.TOTPHeaders(d.session))
				if err != nil {
					return mapErrorToCallToolResult(err), nil, nil
				}
				return nil, map[string]any{"key": a.Key, "payload": v}, nil
			})
		},

		// workouts_edit
		func(s *sdkmcp.Server, d *deps) {
			sdkmcp.AddTool(s, &sdkmcp.Tool{
				Name:        "workouts_edit",
				Description: "Patch workout attributes (PUT /v1/workouts/{key}/attributes). Requires --allow-write.",
			}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, a workoutsEditArgs) (*sdkmcp.CallToolResult, any, error) {
				if e := authGate(d); e != nil {
					return e, nil, nil
				}
				if a.Key == "" {
					return mapErrorToCallToolResult(&api.Error{Code: "USAGE", Message: "key required", Exit: 2}), nil, nil
				}
				v, err := endpoints.EditWorkout(ctx, d.client, a.Key, a.Patch)
				if err != nil {
					return mapErrorToCallToolResult(err), nil, nil
				}
				invalidateWorkoutArtifacts(d, a.Key)
				return nil, map[string]any{"key": a.Key, "payload": v}, nil
			})
		},

		// workouts_batch_update
		func(s *sdkmcp.Server, d *deps) {
			sdkmcp.AddTool(s, &sdkmcp.Tool{
				Name:        "workouts_batch_update",
				Description: "Apply multiple partial workout updates (POST /v1/workouts/batchUpdate). Requires --allow-write.",
			}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, a workoutsBatchUpdateArgs) (*sdkmcp.CallToolResult, any, error) {
				if e := authGate(d); e != nil {
					return e, nil, nil
				}
				v, err := endpoints.BatchUpdate(ctx, d.client, a.Entries)
				if err != nil {
					return mapErrorToCallToolResult(err), nil, nil
				}
				for _, entry := range a.Entries {
					if key, ok := entry["key"].(string); ok {
						invalidateWorkoutArtifacts(d, key)
					}
				}
				return nil, map[string]any{"payload": v}, nil
			})
		},

		// workouts_share
		func(s *sdkmcp.Server, d *deps) {
			sdkmcp.AddTool(s, &sdkmcp.Tool{
				Name:        "workouts_share",
				Description: "Request a signed GPX share URL for a workout (PUT /v1/workouts/{user}/{key}/share/{format}). Requires --allow-write.",
			}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, a workoutsShareArgs) (*sdkmcp.CallToolResult, any, error) {
				if e := authGate(d); e != nil {
					return e, nil, nil
				}
				username := a.Username
				if username == "" {
					username = d.session.Username
				}
				if username == "" {
					return mapErrorToCallToolResult(&api.Error{Code: "USAGE", Message: "username required (session has none)", Exit: 2}), nil, nil
				}
				format := endpoints.ShareFormat(a.Format)
				if format == "" {
					format = endpoints.ShareGPXRoute
				}
				url, err := endpoints.ShareWorkout(ctx, d.client, username, a.Key, format)
				if err != nil {
					return mapErrorToCallToolResult(err), nil, nil
				}
				return nil, map[string]any{"key": a.Key, "url": url}, nil
			})
		},

		// workouts_extensions
		func(s *sdkmcp.Server, d *deps) {
			sdkmcp.AddTool(s, &sdkmcp.Tool{
				Name:        "workouts_extensions",
				Description: "Fetch per-workout extension payloads (POST /v1/workout/extensions/{key}). Despite POST, this is read-shaped — gated as write because cmd parity treats it under the write tier.",
			}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, a workoutsExtensionsArgs) (*sdkmcp.CallToolResult, any, error) {
				if e := authGate(d); e != nil {
					return e, nil, nil
				}
				if bad := unknownExtensionTypes(a.Types); len(bad) > 0 {
					return mapErrorToCallToolResult(&api.Error{
						Code:    "USAGE",
						Message: "unknown extension type(s): " + strings.Join(bad, ", "),
						Hint:    "valid types: " + strings.Join(endpoints.DefaultExtensionTypes, ", "),
						Exit:    2,
					}), nil, nil
				}
				v, err := endpoints.FetchExtensions(ctx, d.client, a.Key, a.Types)
				if err != nil {
					return mapErrorToCallToolResult(err), nil, nil
				}
				return nil, map[string]any{"key": a.Key, "payload": v}, nil
			})
		},

		// workouts_upload — accepts base64, materializes temp files, calls UploadWorkout.
		func(s *sdkmcp.Server, d *deps) {
			sdkmcp.AddTool(s, &sdkmcp.Tool{
				Name:        "workouts_upload",
				Description: "Upload a new workout from base64-encoded SML (and optional extensions JSON) (POST /v1/workout). Requires --allow-write.",
			}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, a workoutsUploadArgs) (*sdkmcp.CallToolResult, any, error) {
				if e := authGate(d); e != nil {
					return e, nil, nil
				}
				if a.SMLBase64 == "" {
					return mapErrorToCallToolResult(&api.Error{Code: "USAGE", Message: "sml_base64 required", Exit: 2}), nil, nil
				}
				smlBytes, err := base64.StdEncoding.DecodeString(a.SMLBase64)
				if err != nil {
					return mapErrorToCallToolResult(&api.Error{Code: "USAGE", Message: "sml_base64 decode: " + err.Error(), Exit: 2}), nil, nil
				}
				smlFile, err := os.CreateTemp("", "suuntool-mcp-*.sml")
				if err != nil {
					return mapErrorToCallToolResult(err), nil, nil
				}
				smlPath := smlFile.Name()
				defer os.Remove(smlPath)
				if _, err := smlFile.Write(smlBytes); err != nil {
					_ = smlFile.Close()
					return mapErrorToCallToolResult(err), nil, nil
				}
				if err := smlFile.Close(); err != nil {
					return mapErrorToCallToolResult(err), nil, nil
				}

				extPath := ""
				if a.ExtensionsBase64 != "" {
					extBytes, err := base64.StdEncoding.DecodeString(a.ExtensionsBase64)
					if err != nil {
						return mapErrorToCallToolResult(&api.Error{Code: "USAGE", Message: "extensions_base64 decode: " + err.Error(), Exit: 2}), nil, nil
					}
					extFile, err := os.CreateTemp("", "suuntool-mcp-*.json")
					if err != nil {
						return mapErrorToCallToolResult(err), nil, nil
					}
					extPath = extFile.Name()
					defer os.Remove(extPath)
					if _, err := extFile.Write(extBytes); err != nil {
						_ = extFile.Close()
						return mapErrorToCallToolResult(err), nil, nil
					}
					if err := extFile.Close(); err != nil {
						return mapErrorToCallToolResult(err), nil, nil
					}
				}

				wkt, err := endpoints.UploadWorkout(ctx, d.client, smlPath, extPath)
				if err != nil {
					return mapErrorToCallToolResult(err), nil, nil
				}
				return nil, wkt, nil
			})
		},

		// guides_upload — accepts base64, sends straight to CreateGuide as
		// bytes.Reader. No temp files: unlike workouts_upload's multipart
		// builder, the guide endpoint takes a raw body, so there's no path
		// requirement to satisfy.
		func(s *sdkmcp.Server, d *deps) {
			sdkmcp.AddTool(s, &sdkmcp.Tool{
				Name: "guides_upload",
				Description: "Upload a new guide from a base64-encoded zip archive (POST /v1/suuntoplus/guides/files). " +
					"suuntool sends the bytes as-is; it does not open or validate the archive. A guide.json externalId " +
					"that collides with an existing guide returns a server error (exit 5) with the server's own " +
					"\"Conflict\" description — there is no dedicated conflict code. Requires --allow-write.",
			}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, a guidesUploadArgs) (*sdkmcp.CallToolResult, any, error) {
				if e := authGate(d); e != nil {
					return e, nil, nil
				}
				if a.ZipBase64 == "" {
					return mapErrorToCallToolResult(&api.Error{Code: "USAGE", Message: "zip_base64 required", Exit: 2}), nil, nil
				}
				zipBytes, err := base64.StdEncoding.DecodeString(a.ZipBase64)
				if err != nil {
					return mapErrorToCallToolResult(&api.Error{Code: "USAGE", Message: "zip_base64 decode: " + err.Error(), Exit: 2}), nil, nil
				}
				v, err := endpoints.CreateGuide(ctx, d.client, bytes.NewReader(zipBytes))
				if err != nil {
					return mapErrorToCallToolResult(err), nil, nil
				}
				return nil, v, nil
			})
		},

		// guides_update
		func(s *sdkmcp.Server, d *deps) {
			sdkmcp.AddTool(s, &sdkmcp.Tool{
				Name: "guides_update",
				Description: "Replace an existing guide's content from a base64-encoded zip archive " +
					"(PUT /v1/suuntoplus/guides/files/{id}). Content only — does not change pinned status; " +
					"use guides_pin/guides_unpin for that. Requires --allow-write.",
			}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, a guidesUpdateArgs) (*sdkmcp.CallToolResult, any, error) {
				if e := authGate(d); e != nil {
					return e, nil, nil
				}
				if a.ZipBase64 == "" {
					return mapErrorToCallToolResult(&api.Error{Code: "USAGE", Message: "zip_base64 required", Exit: 2}), nil, nil
				}
				zipBytes, err := base64.StdEncoding.DecodeString(a.ZipBase64)
				if err != nil {
					return mapErrorToCallToolResult(&api.Error{Code: "USAGE", Message: "zip_base64 decode: " + err.Error(), Exit: 2}), nil, nil
				}
				v, err := endpoints.UpdateGuide(ctx, d.client, a.ID, bytes.NewReader(zipBytes))
				if err != nil {
					return mapErrorToCallToolResult(err), nil, nil
				}
				invalidateGuideArchive(d, a.ID)
				return nil, v, nil
			})
		},

		// guides_pin
		func(s *sdkmcp.Server, d *deps) {
			sdkmcp.AddTool(s, &sdkmcp.Tool{
				Name:        "guides_pin",
				Description: "Pin a guide (PATCH /v1/suuntoplus/guides/items/{id}). Requires --allow-write.",
			}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, a guidesPinArgs) (*sdkmcp.CallToolResult, any, error) {
				if e := authGate(d); e != nil {
					return e, nil, nil
				}
				v, err := endpoints.SetGuidePinned(ctx, d.client, a.ID, true)
				if err != nil {
					return mapErrorToCallToolResult(err), nil, nil
				}
				return nil, v, nil
			})
		},

		// guides_unpin
		func(s *sdkmcp.Server, d *deps) {
			sdkmcp.AddTool(s, &sdkmcp.Tool{
				Name:        "guides_unpin",
				Description: "Unpin a guide (PATCH /v1/suuntoplus/guides/items/{id}). Requires --allow-write.",
			}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, a guidesUnpinArgs) (*sdkmcp.CallToolResult, any, error) {
				if e := authGate(d); e != nil {
					return e, nil, nil
				}
				v, err := endpoints.SetGuidePinned(ctx, d.client, a.ID, false)
				if err != nil {
					return mapErrorToCallToolResult(err), nil, nil
				}
				return nil, v, nil
			})
		},
	}
}

// unknownExtensionTypes returns the subset of types that aren't in the
// canonical list (handoff/WRITE_FLOWS.md §extensions). Empty input → no errors;
// the endpoint substitutes the default set itself.
func unknownExtensionTypes(types []string) []string {
	var bad []string
	for _, t := range types {
		if _, ok := validExtensionTypes[t]; !ok {
			bad = append(bad, t)
		}
	}
	return bad
}
