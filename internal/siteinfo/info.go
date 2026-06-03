package siteinfo

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	alertcallouts "github.com/zmtcreative/gm-alert-callouts"
)

// Info contains startup-loaded site information in source and rendered forms.
type Info struct {
	Markdown   string
	HTML       string
	SourceHash string
}

// Load reads and renders a Markdown information file once.
func Load(path string) (*Info, error) {
	if path == "" {
		return nil, nil
	}

	source, err := os.ReadFile(path) // #nosec G304 -- operator-configured local Markdown file path.
	if err != nil {
		return nil, fmt.Errorf("read site info file: %w", err)
	}

	html, err := RenderMarkdown(source)
	if err != nil {
		return nil, fmt.Errorf("render site info file: %w", err)
	}

	sum := sha256.Sum256(source)

	return &Info{
		Markdown:   string(source),
		HTML:       html,
		SourceHash: hex.EncodeToString(sum[:]),
	}, nil
}

// RenderMarkdown renders Markdown using GFM and GitHub-style alert callouts.
func RenderMarkdown(source []byte) (string, error) {
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			alertcallouts.NewAlertCallouts(
				alertcallouts.UseGFMStrictIcons(),
				alertcallouts.WithFolding(false),
				alertcallouts.WithCustomAlerts(false),
				alertcallouts.WithAllowNOICON(false),
			),
		),
	)

	var buf bytes.Buffer
	if err := md.Convert(source, &buf); err != nil {
		return "", err
	}

	return buf.String(), nil
}
