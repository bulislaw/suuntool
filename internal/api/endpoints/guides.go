// Package endpoints: SuuntoPlus Guides.
//
// suuntool is transport-only for guides: it moves the zip archive (three
// files — manifest.json, guide.json, icon.png) as opaque bytes. It does not
// parse, build, or validate guide.json content — that's a workout-authoring
// concern for whatever produces the archive, not for a CLI meant for
// scripted and agentic API access.
package endpoints

import (
	"context"
	"fmt"
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
	Items []RemoteGuideInfo
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
