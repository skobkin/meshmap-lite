package siteinfo

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadRendersMarkdownAndHashesSource(t *testing.T) {
	t.Parallel()

	source := []byte("# Site info\n\n| A | B |\n|---|---|\n| 1 | 2 |\n\n- [x] ready\n\n> [!WARNING]\n> Read this.\n")
	path := filepath.Join(t.TempDir(), "info.md")
	if err := os.WriteFile(path, source, 0o600); err != nil {
		t.Fatal(err)
	}

	info, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	sum := sha256.Sum256(source)
	if info.SourceHash != hex.EncodeToString(sum[:]) {
		t.Fatalf("unexpected source hash: %s", info.SourceHash)
	}
	if info.Markdown != string(source) {
		t.Fatalf("expected original markdown to be cached")
	}
	for _, want := range []string{"<table>", "type=\"checkbox\"", "callout-warning"} {
		if !strings.Contains(info.HTML, want) {
			t.Fatalf("expected rendered HTML to contain %q, got:\n%s", want, info.HTML)
		}
	}
}

func TestRenderMarkdownSuppressesRawHTML(t *testing.T) {
	t.Parallel()

	html, err := RenderMarkdown([]byte("hello\n\n<script>alert(1)</script>\n\n<div>raw</div>\n"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(html, "<script>") || strings.Contains(html, "<div>raw</div>") {
		t.Fatalf("expected raw HTML to be suppressed, got:\n%s", html)
	}
}
