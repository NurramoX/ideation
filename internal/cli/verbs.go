package cli

// verb is one `idea <verb>`: its help and what it runs.
type verb struct {
	name    string
	usage   string // after `idea `
	summary string // one line for `idea help`
	about   string // the verb's help text
	example string // one agent-oriented example
	flags   []flagSpec
	hidden  bool // left out of help and completion
	// filter: the positionals are Filter words, so an unknown single-dash
	// argument such as `-has:effort` is a negated term, not a flag.
	filter bool
	run    func(*app, *input) int
}

func (v *verb) flag(name string) *flagSpec {
	for i := range v.flags {
		if v.flags[i].name == name {
			return &v.flags[i]
		}
	}
	return nil
}

var (
	jsonFlag    = flagSpec{"--json", "", "print the API's JSON"}
	versionFlag = flagSpec{"--version", "<n>", "write only if the idea is still at Version n"}
	forceFlag   = flagSpec{"--force", "", "write whatever the Version"}
)

// verbs in help order. Filled in init to break the cycle through help.
var verbs []*verb

func init() {
	verbs = []*verb{
		{
			name:    "add",
			usage:   "add <title> [-t <tag>]... [-s <k>=<v>]... [--body-file <path>|-] [--edit] [--json]",
			summary: "capture a new idea",
			about: "Creates an idea and prints its id. The body is empty unless --body-file names a\n" +
				"file, or `-` for stdin; stdin is never read otherwise. --edit opens $EDITOR\n" +
				"first (seeded with --body-file) and creates the idea when you save a change.",
			example: "idea add 'Borrow checker for config files' -t rust -s effort=small --body-file - <<'EOF'\n" +
				"  Config files could be checked like Rust borrows...\n  EOF",
			flags: []flagSpec{
				{"-t", "<tag>", "add a tag (repeatable)"},
				{"-s", "<k>=<v>", "set an attribute (repeatable), status included"},
				{"--body-file", "<path>", "read the body from a file, or - for stdin"},
				{"--edit", "", "write the body in $EDITOR first"},
				jsonFlag,
			},
			run: runAdd,
		},
		{
			name:    "show",
			usage:   "show <id>... [--json]",
			summary: "print ideas with their metadata and body",
			about: "Prints each idea: a header (title, id, status, tags, attributes, version,\n" +
				"dates), a blank line, then the body. Ideas are separated by `---`.\n" +
				"--json prints one envelope per line, with the version to guard a write.",
			example: "idea show 42 --json",
			flags:   []flagSpec{jsonFlag},
			run:     runShow,
		},
		{
			name:    "ls",
			filter:  true,
			usage:   "ls [<filter words>...] [--sort updated|created|reviewed|title|rank] [--asc|--desc] [--limit <n>] [--offset <n>] [-q|--json]",
			summary: "list ideas matching a Filter",
			about: "Lists the ideas matching the Filter, the words joined by spaces. A word\n" +
				"like -has:effort is a negated term; put `--` before one that is also a\n" +
				"flag (-q, -h). Filters: text words, tag:rust,\n" +
				"status:raw,active, key:value, has:key, created:2026-09, reviewed:..90d,\n" +
				"`or`, -negation and (grouping). The default order is rank for text,\n" +
				"otherwise updated, newest first.",
			example: "idea ls borrow checker tag:rust --json",
			flags: []flagSpec{
				{"--sort", "<field>", "updated, created, reviewed, title or rank"},
				{"--asc", "", "ascending order"},
				{"--desc", "", "descending order"},
				{"--limit", "<n>", "show at most n ideas"},
				{"--offset", "<n>", "skip the first n ideas"},
				{"-q", "", "print ids only"},
				jsonFlag,
			},
			run: runLs,
		},
		{
			name:    "body",
			usage:   "body <id>",
			summary: "print an idea's body, byte-exact",
			about:   "Prints the raw markdown body exactly as stored.",
			example: "idea body 42 > /tmp/idea-42.md",
			run:     runBody,
		},
		{
			name:    "write",
			usage:   "write <id> --version <n>|--force [--body-file <path>] [--json]",
			summary: "replace an idea's body",
			about: "Replaces the body with stdin, or with --body-file. Needs the Version you read\n" +
				"(idea show --json) or --force. Empty input writes an empty body.",
			example: "idea write 42 --version 7 --json < body.md",
			flags:   []flagSpec{versionFlag, forceFlag, {"--body-file", "<path>", "read the body from a file, or - for stdin"}, jsonFlag},
			run:     runWrite,
		},
		{
			name:    "replace",
			usage:   "replace <id> <old> <new> [--old-file <path>] [--new-file <path>] [--all] [--version <n>|--force] [--json]",
			summary: "replace text in an idea's body",
			about: "Replaces <old> with <new>, byte-exact. <old> must occur exactly once unless\n" +
				"--all (exit 5 otherwise). An empty <new> deletes. --old-file and --new-file\n" +
				"take the text from files instead of arguments. If the idea changes\n" +
				"meanwhile, it re-reads and retries up to 3 times.",
			example: "idea replace 42 'small effort' 'medium effort' --json",
			flags: []flagSpec{
				{"--old-file", "<path>", "read <old> from a file"},
				{"--new-file", "<path>", "read <new> from a file"},
				{"--all", "", "replace every occurrence"},
				versionFlag, forceFlag, jsonFlag,
			},
			run: runReplace,
		},
		{
			name:    "edit",
			usage:   "edit <id> [--version <n>|--force] [--json]",
			summary: "edit an idea's body in $EDITOR",
			about: "Opens the body in $VISUAL, $EDITOR or vi, and writes it back when you save a\n" +
				"change. If the idea changed meanwhile, asks: [o]verwrite, [r]e-edit or\n" +
				"[a]bort; abort prints your text to stdout.",
			example: "EDITOR=nvim idea edit 42",
			flags:   []flagSpec{versionFlag, forceFlag, jsonFlag},
			run:     runEdit,
		},
		{
			name:    "title",
			usage:   "title <id> <words...> [--version <n>|--force] [--json]",
			summary: "rename an idea",
			about:   "Sets the title to the words joined by single spaces.",
			example: "idea title 42 Borrow checker for config files --version 7",
			flags:   []flagSpec{versionFlag, forceFlag, jsonFlag},
			run:     runTitle,
		},
		{
			name:    "tag",
			usage:   "tag <id> <tag>... [--version <n>] [--json]",
			summary: "add tags",
			about:   "Adds each tag in order, stopping at the first failure.",
			example: "idea tag 42 rust config",
			flags:   []flagSpec{versionFlag, jsonFlag},
			run:     runTag,
		},
		{
			name:    "untag",
			usage:   "untag <id> <tag>... [--version <n>] [--json]",
			summary: "remove tags",
			about:   "Removes each tag in order, stopping at the first failure.",
			example: "idea untag 42 config",
			flags:   []flagSpec{versionFlag, jsonFlag},
			run:     runUntag,
		},
		{
			name:    "set",
			usage:   "set <id> <k>=<v>... [--version <n>] [--json]",
			summary: "set attributes",
			about:   "Sets each attribute in order, splitting on the first `=`, stopping at the\nfirst failure.",
			example: "idea set 42 effort=small area=cli",
			flags:   []flagSpec{versionFlag, jsonFlag},
			run:     runSet,
		},
		{
			name:    "unset",
			usage:   "unset <id> <key>... [--version <n>] [--json]",
			summary: "remove attributes",
			about:   "Removes each attribute in order, stopping at the first failure. Status\ncannot be removed.",
			example: "idea unset 42 effort",
			flags:   []flagSpec{versionFlag, jsonFlag},
			run:     runUnset,
		},
		{
			name:    "status",
			usage:   "status <id> <raw|active|done|dropped> [--version <n>] [--json]",
			summary: "set an idea's Status",
			about:   "Sets the Status: raw and active are open, done and dropped are closed.",
			example: "idea status 42 active",
			flags:   []flagSpec{versionFlag, jsonFlag},
			run:     runStatus,
		},
		{
			name:    "rm",
			usage:   "rm <id>... [-y] [--version <n>|--force]",
			summary: "delete ideas for good",
			about: "Deletes each idea; there is no undo. On a terminal it asks first; without\n" +
				"one it refuses unless -y.",
			example: "! idea rm 42   # agents: hand this line to the user, never run it",
			flags:   []flagSpec{{"-y", "", "do not ask"}, versionFlag, forceFlag},
			run:     runRm,
		},
		{
			name:    "review",
			filter:  true,
			usage:   "review [<filter words>...]",
			summary: "walk ideas in the Review TUI (bare `idea` does this)",
			about: "Opens the Review TUI on the Filter, or on the Review queue\n" +
				"`status:raw,active -reviewed:90d..` when none is given. Press ? inside.",
			example: "idea review tag:rust",
			run:     runReview,
		},
		{
			name:    "reviewed",
			usage:   "reviewed <id>...",
			summary: "mark ideas reviewed",
			about:   "Marks each idea reviewed now. It does not change the Version.",
			example: "idea reviewed 42 43",
			run:     runReviewed,
		},
		{
			name:    "tags",
			usage:   "tags [--json]",
			summary: "list tags with counts",
			about:   "Lists every tag in use with the number of ideas carrying it.",
			example: "idea tags --json",
			flags:   []flagSpec{jsonFlag},
			run:     runTags,
		},
		{
			name:    "attrs",
			usage:   "attrs [<key>] [--json]",
			summary: "list attribute keys, or one key's values, with counts",
			about:   "Lists every attribute key in use, or the values of <key>, with counts.",
			example: "idea attrs effort --json",
			flags:   []flagSpec{jsonFlag},
			run:     runAttrs,
		},
		{
			name:    "daemon",
			usage:   "daemon",
			summary: "run the server in the foreground (launchd runs this)",
			about:   "Serves the API on the socket until SIGTERM or SIGINT. `idea install` has\nlaunchd run it; run it by hand only with IDEATION_HOME set.",
			example: "IDEATION_HOME=/tmp/ideation idea daemon",
			run:     runDaemon,
		},
		{
			name:    "install",
			usage:   "install",
			summary: "install or upgrade the daemon and the agent skill",
			about: "Writes the LaunchAgent, (re)starts the daemon under launchd, waits for it,\n" +
				"and installs the agent skill in ~/.claude/skills/idea. Run it again after\n" +
				"upgrading the binary.",
			example: "idea install",
			run:     runInstall,
		},
		{
			name:    "uninstall",
			usage:   "uninstall",
			summary: "remove the daemon and the agent skill, keeping the ideas",
			about:   "Stops the daemon and removes the LaunchAgent and the skill. The ideas and\nthe logs stay where they are.",
			example: "idea uninstall",
			run:     runUninstall,
		},
		{
			name:    "start",
			usage:   "start",
			summary: "start the installed daemon",
			about:   "Loads the installed LaunchAgent, which starts the daemon.",
			example: "idea start",
			run:     runStart,
		},
		{
			name:    "stop",
			usage:   "stop",
			summary: "stop the daemon",
			about:   "Unloads the LaunchAgent, which stops the daemon until `idea start`. A safe\nwindow to copy the database.",
			example: "idea stop",
			run:     runStop,
		},
		{
			name:    "ping",
			usage:   "ping",
			summary: "check that the daemon answers",
			about:   "Prints the service and api number, or explains why the daemon is down\n(exit 7).",
			example: "idea ping || echo 'ideas are unavailable'",
			run:     runPing,
		},
		{
			name:    "help",
			usage:   "help [<verb>]",
			summary: "show help",
			about:   "Lists the verbs, or explains one.",
			example: "idea help replace",
			run:     runHelp,
		},
		{
			name:    "version",
			usage:   "version",
			summary: "print the version",
			about:   "Prints the binary's version and api, and the daemon's api when it answers.",
			example: "idea version",
			run:     runVersion,
		},
		{
			name:    "completion",
			usage:   "completion fish|zsh|bash",
			summary: "print a shell completion script",
			about:   "Prints the completion script for the shell.",
			example: "idea completion fish > ~/.config/fish/completions/idea.fish",
			run:     runCompletion,
		},
		{
			name:    "complete",
			usage:   "complete <kind> [<prefix>]",
			summary: "completion candidates",
			about:   "Prints completion candidates of a kind: verbs, tags, keys, values <key>, ids.",
			example: "idea complete tags ru",
			hidden:  true,
			run:     runComplete,
		},
	}
}

func lookup(name string) *verb {
	for _, v := range verbs {
		if v.name == name {
			return v
		}
	}
	return nil
}
