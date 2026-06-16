package updatecheck

import "testing"

func TestReleasePostProcessorLinksPlatformReferences(t *testing.T) {
	hash := "ca8aef3abbe7cd9f3357c8f2cfce4fc4dfd34cbd"
	processor := NewReleasePostProcessor(PostProcessOptions{
		RepoURL:     "https://github.com/meshtastic/firmware",
		Repository:  "meshtastic/firmware",
		UserBaseURL: "https://github.com",
	})
	input := "" +
		"Fixed #89 in " + hash + " by @jp-bennett\n" +
		"See https://github.com/meshtastic/firmware/pull/10166 and https://github.com/meshtastic/firmware/issues/10704\n" +
		"Raw commit URL https://github.com/meshtastic/firmware/commit/" + hash + "\n" +
		"`#90 @ignored " + hash + "`\n" +
		"[#91](https://example.invalid/issues/91)\n" +
		"##89 is not an issue reference\n" +
		"```\n#92 @ignored " + hash + "\n```\n"

	got := processor(input)
	want := "" +
		"Fixed [#89](https://github.com/meshtastic/firmware/issues/89) in [ca8aef3abb](https://github.com/meshtastic/firmware/commit/" + hash + ") by [@jp-bennett](https://github.com/jp-bennett)\n" +
		"See [meshtastic/firmware#10166](https://github.com/meshtastic/firmware/pull/10166) and [meshtastic/firmware#10704](https://github.com/meshtastic/firmware/issues/10704)\n" +
		"Raw commit URL https://github.com/meshtastic/firmware/commit/" + hash + "\n" +
		"`#90 @ignored " + hash + "`\n" +
		"[#91](https://example.invalid/issues/91)\n" +
		"##89 is not an issue reference\n" +
		"```\n#92 @ignored " + hash + "\n```\n"
	if got != want {
		t.Fatalf("unexpected post-processed markdown:\nwant: %q\n got: %q", want, got)
	}
}

func TestReleasePostProcessorNoopsWithoutRepo(t *testing.T) {
	processor := NewReleasePostProcessor(PostProcessOptions{})
	input := "Fixed #89 by @jp-bennett"

	if got := processor(input); got != input {
		t.Fatalf("expected no-op processor, got %q", got)
	}
}
