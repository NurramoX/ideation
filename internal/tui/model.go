package tui

import (
	"context"
	"math/rand/v2"
	"os/exec"
	"time"

	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

	"github.com/NurramoX/ideation/internal/api"
	"github.com/NurramoX/ideation/internal/client"
)

// mode is what the keyboard is talking to.
type mode int

const (
	modeList     mode = iota
	modeFilter        // typing in the filter bar
	modeTags          // retagging inline
	modeDelete        // the y/N delete prompt
	modeConflict      // [o]verwrite / [r]e-edit / [a]bort after a 412
	modeTheirs        // their body is in the preview, before re-editing
	modeHelp          // the help screen
)

// row is one line of the snapshot.
type row struct {
	api.Meta
	deleted bool
}

// model is the Review TUI. Its Update is driven by key presses and by the
// results of the commands it issues; every API call runs inside a command.
type model struct {
	ctx context.Context
	c   client.Client

	// Seams for tests.
	now          func() time.Time
	shuffle      func([]row)
	exec         func(*exec.Cmd, tea.ExecCallback) tea.Cmd
	filterDelay  time.Duration
	previewDelay time.Duration

	width, height int
	mode          mode
	showPreview   bool // the single pane shows the preview (narrow terminals)

	bar         textinput.Model
	order       int // index into orders
	listSeq     int // the latest list request; older answers are dropped
	filterSeq   int // the latest filter keystroke, for the debounce
	lastQuery   string
	filterError string // a 400, rendered with a caret
	hint        []string

	rows     []row
	total    int
	sel      int
	offset   int            // first row shown
	reviewed map[int64]bool // marked reviewed this session
	status   string         // the status line's message
	isError  bool           // status is an error

	cache      map[int64]api.Idea // previews by id
	missing    map[int64]string   // preview errors by id
	previewSeq int
	raw        bool
	vp         viewport.Model
	shownID    int64 // the idea whose body is in vp
	shownKey   renderKey
	glam       glamourRenderer

	tags textinput.Model

	edit    *editState
	aborted [][]byte
}

func newModel(ctx context.Context, c client.Client, filter string) *model {
	if filter == "" {
		filter = DefaultFilter
	}
	bar := newInput("/ ")
	bar.SetValue(filter)
	tags := newInput("tags: ")
	return &model{
		ctx:          ctx,
		c:            c,
		now:          time.Now,
		shuffle:      func(rs []row) { rand.Shuffle(len(rs), func(i, j int) { rs[i], rs[j] = rs[j], rs[i] }) },
		exec:         tea.ExecProcess,
		filterDelay:  150 * time.Millisecond,
		previewDelay: 60 * time.Millisecond,
		bar:          bar,
		tags:         tags,
		reviewed:     map[int64]bool{},
		cache:        map[int64]api.Idea{},
		missing:      map[int64]string{},
		vp:           viewport.New(),
		glam:         newGlamourRenderer(),
	}
}

// newInput is a one-line input with a steady cursor.
func newInput(prompt string) textinput.Model {
	in := textinput.New()
	in.Prompt = prompt
	s := in.Styles()
	s.Cursor.Blink = false
	in.SetStyles(s)
	return in
}

func (m *model) Init() tea.Cmd {
	return tea.Batch(m.requery(), tea.RequestBackgroundColor)
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.BackgroundColorMsg:
		m.glam.setDark(msg.IsDark())
	case tea.KeyPressMsg:
		cmd = m.key(msg)
	case listMsg:
		cmd = m.listed(msg)
	case hintMsg:
		if msg.seq == m.listSeq {
			m.hint = msg.lines
		}
	case filterTickMsg:
		if msg.seq == m.filterSeq {
			cmd = m.requery()
		}
	case previewTickMsg:
		cmd = m.fetchPreview(msg)
	case previewMsg:
		m.previewed(msg)
	case reviewedMsg:
		m.markedReviewed(msg)
	case statusMsg:
		m.statusSet(msg)
	case tagsMsg:
		cmd = m.retagged(msg)
	case deletedMsg:
		cmd = m.deleted(msg)
	case editorDoneMsg:
		cmd = m.editorDone(msg)
	case bodyWrittenMsg:
		cmd = m.bodyWritten(msg)
	case theirsMsg:
		m.theirs(msg)
	}
	m.layout()
	return m, cmd
}

// setStatus shows an informational message on the status line.
func (m *model) setStatus(s string) { m.status, m.isError = s, false }

// setError shows err on the status line.
func (m *model) setError(err error) { m.status, m.isError = err.Error(), true }

// after delivers msg once d has passed.
func after(d time.Duration, msg tea.Msg) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg { return msg })
}
