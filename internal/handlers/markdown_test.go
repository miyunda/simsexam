package handlers

import (
	"strings"
	"testing"
)

func TestRenderMarkdownCodeAndUntrustedContent(t *testing.T) {
	source := "Policy 1:\n\n```json\n{\n  \"Action\": \"iam:Get*\",\n  \"Example\": \"<script>alert(1)</script>\"\n}\n```\n\n" +
		"<script>alert(2)</script>\n\n<img src=x onerror=alert(3)>\n\n" +
		"```json\"><img/src=x/onerror=alert(5)>\n{}\n```\n\n" +
		"[unsafe](javascript:alert(4))\n\n![deferred image](https://example.com/image.png)\n\n**Note** with `inline code`."
	got := string(renderMarkdown(source))
	for _, want := range []string{`<pre><code class="language-json">`, "  &quot;Action&quot;: &quot;iam:Get*&quot;", "&lt;script&gt;alert(1)&lt;/script&gt;", "<strong>Note</strong>", "<code>inline code</code>"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %s", want, got)
		}
	}
	for _, unsafe := range []string{"<script", "<img", `href="javascript:`, "onerror="} {
		if strings.Contains(got, unsafe) {
			t.Errorf("unsafe content %q in %s", unsafe, got)
		}
	}
}

func TestRenderMarkdownDisablesExplicitHeadingIDs(t *testing.T) {
	for _, source := range []string{
		"## Title {#x\"><img/src=x/onerror=alert(1)>}",
		"Title {#x\"><img/src=x/onerror=alert(1)>}\n---",
		"## Title {#custom-anchor}",
	} {
		got := string(renderMarkdown(source))
		if strings.Contains(got, "<img") || strings.Contains(got, " id=") {
			t.Errorf("unsafe or unexpected heading rendering: %s", got)
		}
	}
}

func TestRenderMarkdownPreservesOrdinaryHeadings(t *testing.T) {
	if got := string(renderMarkdown("## Title")); !strings.Contains(got, "<h2>Title</h2>") {
		t.Errorf("ordinary heading lost: %s", got)
	}
}
