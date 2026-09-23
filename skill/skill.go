// Package skill embeds the `idea` agent skill (spec §9), which `idea install`
// writes to ~/.claude/skills/idea/SKILL.md.
package skill

import _ "embed"

//go:embed SKILL.md
var Markdown []byte
