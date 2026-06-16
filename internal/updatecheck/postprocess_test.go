package updatecheck

import "testing"

func TestReleasePostProcessorLinksPlatformReferences(t *testing.T) {
	for _, tc := range []struct {
		name       string
		opts       PostProcessOptions
		input      string
		wantOutput string
	}{
		{
			name: "github",
			opts: PostProcessOptions{
				RepoURL:     "https://github.com/meshtastic/firmware",
				Repository:  "meshtastic/firmware",
				UserBaseURL: "https://github.com",
			},
			input: "" +
				"Fixed #89 in ca8aef3abbe7cd9f3357c8f2cfce4fc4dfd34cbd by @jp-bennett\n" +
				"See https://github.com/meshtastic/firmware/pull/10166 and https://github.com/meshtastic/firmware/issues/10704\n" +
				"Raw commit URL https://github.com/meshtastic/firmware/commit/ca8aef3abbe7cd9f3357c8f2cfce4fc4dfd34cbd\n" +
				"`#90 @ignored ca8aef3abbe7cd9f3357c8f2cfce4fc4dfd34cbd`\n" +
				"[#91](https://example.invalid/issues/91)\n" +
				"##89 is not an issue reference\n" +
				"```\n#92 @ignored ca8aef3abbe7cd9f3357c8f2cfce4fc4dfd34cbd\n```\n",
			wantOutput: "" +
				"Fixed [#89](https://github.com/meshtastic/firmware/issues/89) in [ca8aef3abb](https://github.com/meshtastic/firmware/commit/ca8aef3abbe7cd9f3357c8f2cfce4fc4dfd34cbd) by [@jp-bennett](https://github.com/jp-bennett)\n" +
				"See [meshtastic/firmware#10166](https://github.com/meshtastic/firmware/pull/10166) and [meshtastic/firmware#10704](https://github.com/meshtastic/firmware/issues/10704)\n" +
				"Raw commit URL https://github.com/meshtastic/firmware/commit/ca8aef3abbe7cd9f3357c8f2cfce4fc4dfd34cbd\n" +
				"`#90 @ignored ca8aef3abbe7cd9f3357c8f2cfce4fc4dfd34cbd`\n" +
				"[#91](https://example.invalid/issues/91)\n" +
				"##89 is not an issue reference\n" +
				"```\n#92 @ignored ca8aef3abbe7cd9f3357c8f2cfce4fc4dfd34cbd\n```\n",
		},
		{
			name: "forgejo",
			opts: PostProcessOptions{
				RepoURL:     "https://git.example.org/skobkin/meshmap-lite",
				Repository:  "skobkin/meshmap-lite",
				UserBaseURL: "https://git.example.org",
			},
			input: "" +
				"Fixed #89 in 84ef31f017699e063c9a455452e78f41619b6fe2 by @skobkin\n" +
				"See https://git.example.org/skobkin/meshmap-lite/pull/15 and https://git.example.org/skobkin/meshmap-lite/issues/16\n" +
				"Raw commit URL https://git.example.org/skobkin/meshmap-lite/commit/84ef31f017699e063c9a455452e78f41619b6fe2\n",
			wantOutput: "" +
				"Fixed [#89](https://git.example.org/skobkin/meshmap-lite/issues/89) in [84ef31f017](https://git.example.org/skobkin/meshmap-lite/commit/84ef31f017699e063c9a455452e78f41619b6fe2) by [@skobkin](https://git.example.org/skobkin)\n" +
				"See [skobkin/meshmap-lite#15](https://git.example.org/skobkin/meshmap-lite/pull/15) and [skobkin/meshmap-lite#16](https://git.example.org/skobkin/meshmap-lite/issues/16)\n" +
				"Raw commit URL https://git.example.org/skobkin/meshmap-lite/commit/84ef31f017699e063c9a455452e78f41619b6fe2\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			processor := NewReleasePostProcessor(tc.opts)

			got := processor(tc.input)
			if got != tc.wantOutput {
				t.Fatalf("unexpected post-processed markdown:\nwant: %q\n got: %q", tc.wantOutput, got)
			}
		})
	}
}

func TestReleasePostProcessorNoopsWithoutRepo(t *testing.T) {
	processor := NewReleasePostProcessor(PostProcessOptions{})
	input := "Fixed #89 by @jp-bennett"

	if got := processor(input); got != input {
		t.Fatalf("expected no-op processor, got %q", got)
	}
}
