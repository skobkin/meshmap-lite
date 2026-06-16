package updatecheck

import (
	"regexp"
	"sort"
	"strings"
)

const commitDisplayLength = 10

var (
	commitHashRE = regexp.MustCompile(`\b[0-9a-fA-F]{40}\b`)
	issueRefRE   = regexp.MustCompile(`(^|[^\w/\\#])#([1-9][0-9]*)\b`)
	usernameRE   = regexp.MustCompile(`(^|[^\w/\\@])@([A-Za-z0-9][A-Za-z0-9_.-]*)\b`)
	protectedRE  = regexp.MustCompile("`[^`\\n]*`|\\[[^\\]\\n]*\\]\\([^\\)\\n]*\\)")
	bareURLRE    = regexp.MustCompile(`https?://[^\s)]+`)
	fenceRE      = regexp.MustCompile(`^\s*(` + "```" + `|~~~)`)
)

// ReleasePostProcessor normalizes upstream release Markdown before it is cached.
type ReleasePostProcessor func(markdown string) string

// PostProcessOptions describes a platform repository for release Markdown
// normalization.
type PostProcessOptions struct {
	RepoURL     string
	Repository  string
	UserBaseURL string
}

// NewReleasePostProcessor builds a Markdown post-processor for platform-specific
// repository links. Empty or invalid options produce a no-op processor.
func NewReleasePostProcessor(opts PostProcessOptions) ReleasePostProcessor {
	repoURL := strings.TrimRight(strings.TrimSpace(opts.RepoURL), "/")
	repository := strings.Trim(strings.TrimSpace(opts.Repository), "/")
	userBaseURL := strings.TrimRight(strings.TrimSpace(opts.UserBaseURL), "/")
	if repoURL == "" || repository == "" {
		return func(markdown string) string { return markdown }
	}
	if userBaseURL == "" {
		userBaseURL = repoURL
		if slash := strings.LastIndex(userBaseURL, "/"); slash > len("https://") {
			userBaseURL = userBaseURL[:slash]
		}
	}

	issueURLRE := regexp.MustCompile(regexp.QuoteMeta(repoURL) + `/(issues|pull)/([1-9][0-9]*)\b`)

	return func(markdown string) string {
		return postProcessReleaseMarkdown(markdown, repoURL, repository, userBaseURL, issueURLRE)
	}
}

func postProcessReleaseMarkdown(markdown, repoURL, repository, userBaseURL string, issueURLRE *regexp.Regexp) string {
	if markdown == "" {
		return markdown
	}

	lines := strings.SplitAfter(markdown, "\n")
	var out strings.Builder
	out.Grow(len(markdown))

	inFence := false
	for _, line := range lines {
		if fenceRE.MatchString(line) {
			inFence = !inFence
			out.WriteString(line)

			continue
		}
		if inFence {
			out.WriteString(line)

			continue
		}

		out.WriteString(postProcessReleaseMarkdownLine(line, repoURL, repository, userBaseURL, issueURLRE))
	}

	return out.String()
}

func postProcessReleaseMarkdownLine(line, repoURL, repository, userBaseURL string, issueURLRE *regexp.Regexp) string {
	ranges := protectedRanges(line)
	if len(ranges) == 0 {
		return postProcessReleaseMarkdownSegment(line, repoURL, repository, userBaseURL, issueURLRE)
	}

	var out strings.Builder
	out.Grow(len(line))
	pos := 0
	for _, r := range ranges {
		if r.start > pos {
			out.WriteString(postProcessReleaseMarkdownSegment(line[pos:r.start], repoURL, repository, userBaseURL, issueURLRE))
		}
		out.WriteString(line[r.start:r.end])
		pos = r.end
	}
	if pos < len(line) {
		out.WriteString(postProcessReleaseMarkdownSegment(line[pos:], repoURL, repository, userBaseURL, issueURLRE))
	}

	return out.String()
}

type textRange struct {
	start int
	end   int
}

func protectedRanges(line string) []textRange {
	matches := protectedRE.FindAllStringIndex(line, -1)
	if len(matches) == 0 {
		return nil
	}

	ranges := make([]textRange, 0, len(matches))
	for _, match := range matches {
		ranges = append(ranges, textRange{start: match[0], end: match[1]})
	}
	sort.Slice(ranges, func(i, j int) bool {
		if ranges[i].start == ranges[j].start {
			return ranges[i].end < ranges[j].end
		}

		return ranges[i].start < ranges[j].start
	})

	merged := ranges[:0]
	for _, r := range ranges {
		if len(merged) == 0 || r.start > merged[len(merged)-1].end {
			merged = append(merged, r)

			continue
		}
		if r.end > merged[len(merged)-1].end {
			merged[len(merged)-1].end = r.end
		}
	}

	return merged
}

func postProcessReleaseMarkdownSegment(segment, repoURL, repository, userBaseURL string, issueURLRE *regexp.Regexp) string {
	ranges := bareURLRE.FindAllStringIndex(segment, -1)
	if len(ranges) == 0 {
		return postProcessReleaseText(segment, repoURL, userBaseURL)
	}

	var out strings.Builder
	out.Grow(len(segment))
	pos := 0
	for _, r := range ranges {
		if r[0] > pos {
			out.WriteString(postProcessReleaseText(segment[pos:r[0]], repoURL, userBaseURL))
		}
		out.WriteString(postProcessReleaseURL(segment[r[0]:r[1]], repository, issueURLRE))
		pos = r[1]
	}
	if pos < len(segment) {
		out.WriteString(postProcessReleaseText(segment[pos:], repoURL, userBaseURL))
	}

	return out.String()
}

func postProcessReleaseText(text, repoURL, userBaseURL string) string {
	withCommits := commitHashRE.ReplaceAllStringFunc(text, func(hash string) string {
		return "[" + hash[:commitDisplayLength] + "](" + repoURL + "/commit/" + hash + ")"
	})
	withIssues := issueRefRE.ReplaceAllString(withCommits, "${1}[#${2}]("+repoURL+"/issues/${2})")

	return usernameRE.ReplaceAllString(withIssues, "${1}[@${2}]("+userBaseURL+"/${2})")
}

func postProcessReleaseURL(rawURL, repository string, issueURLRE *regexp.Regexp) string {
	match := issueURLRE.FindStringSubmatchIndex(rawURL)
	if match == nil || match[0] != 0 {
		return rawURL
	}
	numberStart, numberEnd := match[4], match[5]
	if numberStart < 0 || numberEnd < 0 {
		return rawURL
	}

	linkURL := rawURL[:match[1]]
	suffix := rawURL[match[1]:]

	return "[" + repository + "#" + rawURL[numberStart:numberEnd] + "](" + linkURL + ")" + suffix
}
