package cli

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/NurramoX/ideation/internal/api"
)

// render prints an idea for humans: a header block, a blank line, the body.
func (a *app) render(idea api.Idea, st style) {
	var b strings.Builder
	row := func(label, value string) {
		fmt.Fprintf(&b, "%s%s\n", st.dim(fmt.Sprintf("%-12s", label)), value)
	}
	row("title", st.bold(idea.Title))
	row("id", strconv.FormatInt(idea.ID, 10))
	row("status", st.status(idea.Attributes[api.StatusKey]))
	if len(idea.Tags) > 0 {
		row("tags", strings.Join(idea.Tags, " "))
	}
	label := "attributes"
	for _, k := range attributeKeys(idea.Attributes) {
		row(label, k+"="+idea.Attributes[k])
		label = ""
	}
	row("version", strconv.FormatInt(idea.Version, 10))
	row("created", when(&idea.CreatedAt))
	row("updated", when(&idea.UpdatedAt))
	row("reviewed", when(idea.ReviewedAt))
	b.WriteString("\n")
	b.WriteString(idea.Body)
	if idea.Body != "" && !strings.HasSuffix(idea.Body, "\n") {
		b.WriteString("\n")
	}
	fmt.Fprint(a.stdout, b.String())
}

// attributeKeys are the keys other than status, sorted.
func attributeKeys(attrs map[string]string) []string {
	var ks []string
	for k := range attrs {
		if k != api.StatusKey {
			ks = append(ks, k)
		}
	}
	slices.Sort(ks)
	return ks
}

// when shows an instant in local time, "never" for none.
func when(t *api.Time) string {
	if t == nil {
		return "never"
	}
	return t.Local().Format("2006-01-02 15:04")
}

// table prints rows as left-aligned columns separated by two spaces. Widths
// count runes of the uncoloured text; paint colours a cell after padding.
func table(rows [][]string, paint func(col int, cell string) string) string {
	var widths []int
	for _, r := range rows {
		for i, c := range r {
			if i == len(widths) {
				widths = append(widths, 0)
			}
			widths[i] = max(widths[i], utf8.RuneCountInString(c))
		}
	}
	var b strings.Builder
	for _, r := range rows {
		var line strings.Builder
		for i, c := range r {
			if i > 0 {
				line.WriteString("  ")
			}
			line.WriteString(paint(i, c))
			if i < len(r)-1 {
				line.WriteString(strings.Repeat(" ", widths[i]-utf8.RuneCountInString(c)))
			}
		}
		b.WriteString(strings.TrimRight(line.String(), " "))
		b.WriteString("\n")
	}
	return b.String()
}
