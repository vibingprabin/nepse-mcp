// Command ansucorpus dumps the full Ansu Invest research article archive to
// disk as plain-text markdown files (one per article), so the corpus can be
// read and synthesized by subagents without holding the raw HTML in context.
//
// Usage: go run ./cmd/ansucorpus -out <dir> [-limit N]
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"vibinprabin/nepse-mcp/ansu"
)

func main() {
	out := flag.String("out", ".sisyphus/ansu-corpus", "output directory")
	limit := flag.Int("limit", 0, "max articles to dump (0 = all)")
	flag.Parse()

	c := ansu.NewClient()

	var all []ansu.Article
	seen := map[string]bool{}
	page := 1
	for {
		items, err := c.ListArticles("", page, 100)
		if err != nil {
			fmt.Fprintf(os.Stderr, "list page %d: %v\n", page, err)
			break
		}
		if len(items) == 0 {
			break
		}
		added := 0
		for _, it := range items {
			if !seen[it.Slug] {
				seen[it.Slug] = true
				all = append(all, it)
				added++
			}
		}
		if added == 0 && page > 5 {
			break
		}
		if *limit > 0 && len(all) >= *limit {
			break
		}
		page++
		time.Sleep(200 * time.Millisecond)
	}
	fmt.Printf("enumerated %d unique articles (%d pages)\n", len(all), page)

	if err := os.MkdirAll(*out, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "mkdir:", err)
		os.Exit(1)
	}

	ok, fail := 0, 0
	for i, a := range all {
		if *limit > 0 && i >= *limit {
			break
		}
		d, related, err := c.GetArticle(a.Slug)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  get %s: %v\n", a.Slug, err)
			fail++
			continue
		}
		body := ansu.HTMLToText(d.Description)
		if strings.TrimSpace(body) == "" {
			body = ansu.HTMLToText(d.Summary)
		}
		var sb strings.Builder
		fmt.Fprintf(&sb, "# %s\n", d.Title)
		if d.SubTitle != "" {
			fmt.Fprintf(&sb, "_%s_\n", d.SubTitle)
		}
		premium := ""
		if d.IsPremium == 1 {
			premium = " 🔒 premium"
		}
		fmt.Fprintf(&sb, "**Posted:** %s%s\n\n", d.PostedAt, premium)
		if d.Disclaimer != "" {
			fmt.Fprintf(&sb, "_%s_\n\n", ansu.HTMLToText(d.Disclaimer))
		}
		if body != "" {
			sb.WriteString(body)
			sb.WriteString("\n\n")
		}
		if len(related) > 0 {
			sb.WriteString("## Related\n")
			for _, r := range related {
				fmt.Fprintf(&sb, "- %s — slug: `%s`\n", r.Title, r.Slug)
			}
			sb.WriteString("\n")
		}
		fn := filepath.Join(*out, sanitize(d.Slug)+".md")
		if err := os.WriteFile(fn, []byte(sb.String()), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "  write %s: %v\n", fn, err)
			fail++
			continue
		}
		ok++
		if i%25 == 0 {
			fmt.Printf("  %d/%d dumped (%d ok, %d fail)\n", i+1, len(all), ok, fail)
		}
		time.Sleep(150 * time.Millisecond)
	}
	fmt.Printf("done: %d ok, %d failed → %s\n", ok, fail, *out)
}

func sanitize(s string) string {
	var sb strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			sb.WriteRune(r)
		default:
			sb.WriteRune('-')
		}
	}
	return strings.Trim(sb.String(), "-")
}
