package output_test

import (
	"bytes"
	"crypto/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bulislaw/suuntool/internal/output"
)

type sample struct {
	Name  string `json:"name"`
	Score int    `json:"score"`
}

func (s sample) Pretty() string {
	return "name=" + s.Name
}

func TestRender_JSON_PrettyPrintsToWriter(t *testing.T) {
	var buf bytes.Buffer
	err := output.Render(&buf, sample{"x", 1}, output.Opts{Format: "json"})
	require.NoError(t, err)
	assert.Contains(t, buf.String(), `"name": "x"`)
}

func TestRender_Pretty_UsesPrettyMethod(t *testing.T) {
	var buf bytes.Buffer
	err := output.Render(&buf, sample{"x", 1}, output.Opts{Format: "pretty"})
	require.NoError(t, err)
	assert.Equal(t, "name=x\n", buf.String())
}

func TestRender_FormatFromExtension(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.json")
	err := output.RenderToFile(path, sample{"x", 1}, output.Opts{Format: "auto"})
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.True(t, strings.Contains(string(data), `"name": "x"`))
}

type tabularSample struct{}

func (tabularSample) Table() ([]string, [][]string) {
	return []string{"a", "b"}, [][]string{
		{"1", "two"},
		{"has\ttab", "has\nnewline"},
	}
}

func (tabularSample) Pretty() string { return "should not be used" }

func TestRender_TSV_EmitsHeadersAndScrubbedCells(t *testing.T) {
	var buf bytes.Buffer
	err := output.Render(&buf, tabularSample{}, output.Opts{Format: "tsv"})
	require.NoError(t, err)
	assert.Equal(t, "a\tb\n1\ttwo\nhas tab\thas newline\n", buf.String())
}

func TestRender_TSV_FallsBackToJSONForNonTabular(t *testing.T) {
	var buf bytes.Buffer
	err := output.Render(&buf, sample{"x", 1}, output.Opts{Format: "tsv"})
	require.NoError(t, err)
	assert.Contains(t, buf.String(), `"name": "x"`)
}

func TestRenderToFile_TSVFromExtension(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.tsv")
	err := output.RenderToFile(path, tabularSample{}, output.Opts{Format: "auto"})
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "a\tb\n1\ttwo\nhas tab\thas newline\n", string(data))
}

func TestRender_FieldsProjectsArrayAndForcesJSON(t *testing.T) {
	var buf bytes.Buffer
	items := []sample{{"x", 1}, {"y", 2}}
	// IsTTY+Format=pretty would normally call Pretty(); --fields overrides it.
	err := output.Render(&buf, items, output.Opts{Format: "pretty", IsTTY: true, Fields: []string{"name"}})
	require.NoError(t, err)
	out := buf.String()
	assert.Contains(t, out, `"name": "x"`)
	assert.NotContains(t, out, `"score"`)
	assert.NotContains(t, out, "name=") // Pretty() output is bypassed
}

func TestWriteRaw_StreamsToFile(t *testing.T) {
	payload := make([]byte, 1024)
	_, err := rand.Read(payload)
	require.NoError(t, err)

	dir := t.TempDir()
	path := filepath.Join(dir, "raw.bin")

	err = output.WriteRaw(path, bytes.NewReader(payload))
	require.NoError(t, err)

	got, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, payload, got)
}
