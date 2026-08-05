// Package endpoints: SuuntoPlus Guides.
//
// suuntool is transport-only for guides: it moves the zip archive (three
// files — manifest.json, guide.json, icon.png) as opaque bytes. It does not
// parse, build, or validate guide.json content — that's a workout-authoring
// concern for whatever produces the archive, not for a CLI meant for
// scripted and agentic API access.
package endpoints

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/tajchert/suuntool/internal/api"
)

// RemoteGuideInfo is a guide's metadata, as returned by list/create/update.
// Note there is no externalId field here — the server enforces uniqueness on
// one (a duplicate create returns 409), but it is never echoed back on this
// endpoint family. That's a real property of the wire response, not an
// omission to fix.
type RemoteGuideInfo struct {
	ID                   string `json:"id"`
	CatalogueID          string `json:"catalogueId,omitempty"`
	FileModificationTime int64  `json:"fileModificationTime"`
	Name                 string `json:"name"`
	Owner                string `json:"owner"`
	OwnerID              string `json:"ownerId,omitempty"`
	Description          string `json:"description,omitempty"`
	ShortDescription     string `json:"shortDescription,omitempty"`
	RichText             string `json:"richText,omitempty"`
	LocalDate            string `json:"localDate,omitempty"` // yyyy-MM-dd
	URL                  string `json:"url,omitempty"`
	IconURL              string `json:"iconUrl,omitempty"`
	BackgroundURL        string `json:"backgroundUrl,omitempty"`
	Activities           []int  `json:"activities,omitempty"`
	Pinned               bool   `json:"pinned"`
}

// Pretty renders a single guide as key/value lines, matching the convention
// for single-record responses elsewhere in this package.
func (g RemoteGuideInfo) Pretty() string {
	modified := time.UnixMilli(g.FileModificationTime).Local().Format("2006-01-02 15:04")
	lines := []string{
		fmt.Sprintf("id:               %s", g.ID),
		fmt.Sprintf("name:             %s", g.Name),
		fmt.Sprintf("owner:            %s", g.Owner),
		fmt.Sprintf("modified:         %s", modified),
		fmt.Sprintf("pinned:           %v", g.Pinned),
	}
	if g.ShortDescription != "" {
		lines = append(lines, fmt.Sprintf("shortDescription: %s", g.ShortDescription))
	}
	if g.LocalDate != "" {
		lines = append(lines, fmt.Sprintf("localDate:        %s", g.LocalDate))
	}
	if g.CatalogueID != "" {
		lines = append(lines, fmt.Sprintf("catalogueId:      %s", g.CatalogueID))
	}
	if len(g.Activities) > 0 {
		lines = append(lines, fmt.Sprintf("activities:       %v", g.Activities))
	}
	out := lines[0]
	for _, l := range lines[1:] {
		out += "\n" + l
	}
	return out
}

// GuideList wraps a page of guides. The endpoint takes no pagination
// parameters — the mobile client fetches everything in one call — so unlike
// WorkoutList there is no cursor field here.
type GuideList struct {
	Items []RemoteGuideInfo `json:"items"`
}

// Table renders the list as headers/rows, so --format tsv works and Pretty
// can reuse it via renderTable — the convention CLAUDE.md sets for
// list-shaped responses.
func (l GuideList) Table() ([]string, [][]string) {
	headers := []string{"Modified", "Name", "Pinned", "LocalDate", "ID"}
	rows := make([][]string, 0, len(l.Items))
	for _, g := range l.Items {
		rows = append(rows, []string{
			time.UnixMilli(g.FileModificationTime).Local().Format("2006-01-02 15:04"),
			g.Name,
			fmt.Sprintf("%v", g.Pinned),
			g.LocalDate,
			g.ID,
		})
	}
	return headers, rows
}

func (l GuideList) Pretty() string {
	headers, rows := l.Table()
	footer := fmt.Sprintf("\n%d %s", len(l.Items), pluralGuide(len(l.Items)))
	return renderTable(headers, rows) + footer
}

func pluralGuide(n int) string {
	if n == 1 {
		return "guide"
	}
	return "guides"
}

// UpdatePinnedStatusBody is the request body for SetGuidePinned. id
// duplicates the path parameter — that's the real wire shape, mirrored
// faithfully rather than simplified.
type UpdatePinnedStatusBody struct {
	ID     string `json:"id"`
	Pinned bool   `json:"pinned"`
}

// SetGuidePinned pins or unpins a guide (PATCH suuntoplus/guides/items/{id}).
// This is the only way to change pinned status — UpdateGuide's content PUT
// does not touch it. Confirmed live: pinning sets `pinned` in the response
// and moves the guide to the front of the account's priority order.
func SetGuidePinned(ctx context.Context, c *api.Client, id string, pinned bool) (*RemoteGuideInfo, error) {
	body, err := json.Marshal(UpdatePinnedStatusBody{ID: id, Pinned: pinned})
	if err != nil {
		return nil, &api.Error{Code: "USAGE", Message: err.Error(), Exit: 2}
	}
	b, err := c.Do(ctx, "PATCH", "suuntoplus/guides/items/"+id, bytes.NewReader(body),
		map[string]string{"Content-Type": "application/json"})
	if err != nil {
		return nil, err
	}
	g, err := api.DecodeAsko[RemoteGuideInfo](b)
	if err != nil {
		return nil, err
	}
	return &g, nil
}

// RemoteGuidePriorityEntry is one entry in the account's guide priority order.
type RemoteGuidePriorityEntry struct {
	ID string `json:"id"`
}

// RemoteGuidePriorities is the priority ordering of the account's guides.
type RemoteGuidePriorities struct {
	Guides []RemoteGuidePriorityEntry `json:"guides"`
}

// Table renders the ordering as rank/id rows. Rank is the 1-based position in
// the array — the server conveys priority by position, not by an explicit
// field, so it is derived here rather than read off the wire.
func (p RemoteGuidePriorities) Table() ([]string, [][]string) {
	headers := []string{"Rank", "ID"}
	rows := make([][]string, 0, len(p.Guides))
	for i, g := range p.Guides {
		rows = append(rows, []string{fmt.Sprintf("%d", i+1), g.ID})
	}
	return headers, rows
}

func (p RemoteGuidePriorities) Pretty() string {
	headers, rows := p.Table()
	footer := fmt.Sprintf("\n%d %s", len(p.Guides), pluralGuide(len(p.Guides)))
	return renderTable(headers, rows) + footer
}

// GuidePriority fetches the account's guide priority order
// (GET suuntoplus/guides/priority). Confirmed live: returns every guide on
// the account as {id}, ordered — most recently pinned first.
func GuidePriority(ctx context.Context, c *api.Client) (*RemoteGuidePriorities, error) {
	b, err := c.Do(ctx, "GET", "suuntoplus/guides/priority", nil, nil)
	if err != nil {
		return nil, err
	}
	p, err := api.DecodeAsko[RemoteGuidePriorities](b)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// DeleteGuide permanently removes a guide
// (DELETE suuntoplus/guides/files/{id}). Cannot be undone.
func DeleteGuide(ctx context.Context, c *api.Client, id string) error {
	_, err := c.Do(ctx, "DELETE", "suuntoplus/guides/files/"+id, nil, nil)
	return err
}

// ListGuides fetches every guide on the account (GET suuntoplus/guides/items).
// No offset/limit/since — the server accepts none for this endpoint.
func ListGuides(ctx context.Context, c *api.Client) (*GuideList, error) {
	body, err := c.Do(ctx, "GET", "suuntoplus/guides/items", nil, nil)
	if err != nil {
		return nil, err
	}
	items, err := api.DecodeAsko[[]RemoteGuideInfo](body)
	if err != nil {
		return nil, err
	}
	return &GuideList{Items: items}, nil
}

// guideClientID is a static, app-wide header value required on guide writes.
// Extracted from APK com.stt.android.suunto v6.11.8's Retrofit @Headers
// annotation on the guides upload/update calls. Not per-account; on a Suunto
// app major version bump it may need refreshing, same as the signing keys in
// internal/auth/keys.go.
const guideClientID = "5c2fa984-4425-4e72-8f7c-deeaa454b9c6"

func guideWriteHeaders() map[string]string {
	return map[string]string{
		"Content-Type": "application/zip",
		"Client-Id":    guideClientID,
	}
}

// CreateGuide uploads a new guide archive (POST suuntoplus/guides/files). body
// is the raw zip bytes — three files, manifest.json + guide.json + icon.png —
// sent as-is; suuntool does not open or validate it. No x-totp is required
// here, unlike comments/reactions/edits.
//
// A guide.json externalId that collides with an existing guide on the account
// returns 409, which Do() maps to Code:"SERVER", Exit:5 (there is no
// dedicated CONFLICT code in this client — see the exit-code table in
// CLAUDE.md). The server's own "Conflict" description is in the message.
//
// The response's owner field comes back "Suunto" regardless of what the
// manifest sent — confirmed live, not a fluke. UpdateGuide's response, by
// contrast, reflects the owner actually sent. Likely explanation: the server
// stamps owner from the authenticated client's identity on create (every
// caller here presents as the same first-party app identity, so it's always
// "Suunto"), while update only touches content and leaves the already-stored
// owner alone. Unconfirmed — this API has no docs to check it against — but
// it fits the asymmetry.
func CreateGuide(ctx context.Context, c *api.Client, body io.Reader) (*RemoteGuideInfo, error) {
	b, err := c.Do(ctx, "POST", "suuntoplus/guides/files", body, guideWriteHeaders())
	if err != nil {
		return nil, err
	}
	g, err := api.DecodeAsko[RemoteGuideInfo](b)
	if err != nil {
		return nil, err
	}
	return &g, nil
}

// UpdateGuide replaces an existing guide's content
// (PUT suuntoplus/guides/files/{id}). Content only — it does not change
// pinned status or ownership; use SetGuidePinned for that.
func UpdateGuide(ctx context.Context, c *api.Client, id string, body io.Reader) (*RemoteGuideInfo, error) {
	b, err := c.Do(ctx, "PUT", "suuntoplus/guides/files/"+id, body, guideWriteHeaders())
	if err != nil {
		return nil, err
	}
	g, err := api.DecodeAsko[RemoteGuideInfo](b)
	if err != nil {
		return nil, err
	}
	return &g, nil
}

// DownloadGuide streams the guide's zip archive
// (GET suuntoplus/guides/files/{id}). The server reconstitutes the archive
// from what it has stored rather than echoing the upload byte-for-byte — it
// comes back with manifest.json dropped and JSON numbers renormalized to
// floats — so treat this as "an equivalent archive", not "the same bytes you
// sent". Caller MUST Close.
func DownloadGuide(ctx context.Context, c *api.Client, id string) (io.ReadCloser, error) {
	return c.DoStream(ctx, "GET", "suuntoplus/guides/files/"+id, nil, nil)
}
