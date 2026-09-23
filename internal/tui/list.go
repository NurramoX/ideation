package tui

import (
	"context"

	tea "charm.land/bubbletea/v2"

	"github.com/NurramoX/ideation/internal/api"
	"github.com/NurramoX/ideation/internal/client"
	"github.com/NurramoX/ideation/internal/filter"
	"github.com/NurramoX/ideation/internal/hint"
)

// ordering is one step of the `o` cycle.
type ordering struct {
	name  string
	sort  string // "" leaves the order to the server, then shuffles
	order string
}

var orders = []ordering{
	{name: "updated", sort: "updated", order: "desc"},
	{name: "reviewed", sort: "reviewed", order: "asc"},
	{name: "created", sort: "created", order: "desc"},
	{name: "random"},
}

// listMsg is the answer to a requery.
type listMsg struct {
	seq    int
	filter string
	list   api.List
	err    error
}

// hintMsg is the vocabulary hint for an empty result.
type hintMsg struct {
	seq   int
	lines []string
}

// requery lists the filter in the bar, in the current order. Only the answer
// to the latest requery is applied.
func (m *model) requery() tea.Cmd {
	m.listSeq++
	seq, f, o := m.listSeq, m.bar.Value(), orders[m.order]
	m.lastQuery = f
	ctx, c := m.ctx, m.c
	return func() tea.Msg {
		l, err := c.List(ctx, client.ListParams{Filter: f, Sort: o.sort, Order: o.order})
		return listMsg{seq: seq, filter: f, list: l, err: err}
	}
}

// listed replaces the snapshot with a requery's answer, keeping the selected
// idea when it is still present.
func (m *model) listed(msg listMsg) tea.Cmd {
	if msg.seq != m.listSeq {
		return nil
	}
	if msg.err != nil {
		if pos, detail, ok := client.FilterError(msg.err); ok {
			m.filterError = filter.Caret(msg.filter, pos) + "\n" + detail
			return nil
		}
		m.setError(msg.err)
		return nil
	}
	m.filterError = ""
	m.hint = nil
	keep, hadSel := m.selectedID()
	m.rows = make([]row, len(msg.list.Ideas))
	for i, meta := range msg.list.Ideas {
		m.rows[i] = row{Meta: meta}
	}
	if orders[m.order].sort == "" {
		m.shuffle(m.rows)
	}
	m.total = msg.list.Total
	m.sel, m.offset = 0, 0
	if hadSel {
		if i := m.indexOf(keep); i >= 0 {
			m.sel = i
		}
	}
	if len(m.rows) == 0 {
		return m.vocabularyHint(msg.seq, msg.filter)
	}
	return m.loadPreview()
}

func (m *model) vocabularyHint(seq int, f string) tea.Cmd {
	ctx, c := m.ctx, m.c
	return func() tea.Msg {
		lines, _ := hint.Vocabulary(ctx, c, f)
		return hintMsg{seq: seq, lines: lines}
	}
}

// selectedID is the id of the selected row.
func (m *model) selectedID() (int64, bool) {
	if m.sel < 0 || m.sel >= len(m.rows) {
		return 0, false
	}
	return m.rows[m.sel].ID, true
}

func (m *model) selectedRow() *row {
	if m.sel < 0 || m.sel >= len(m.rows) {
		return nil
	}
	return &m.rows[m.sel]
}

func (m *model) indexOf(id int64) int {
	for i, r := range m.rows {
		if r.ID == id {
			return i
		}
	}
	return -1
}

// updateRow changes the row of id in place, if the snapshot has it.
func (m *model) updateRow(id int64, f func(*row)) {
	if i := m.indexOf(id); i >= 0 {
		f(&m.rows[i])
	}
}

// selectRow moves the selection to i, clamped, and loads its preview.
func (m *model) selectRow(i int) tea.Cmd {
	if len(m.rows) == 0 {
		return nil
	}
	i = max(0, min(i, len(m.rows)-1))
	if i == m.sel {
		return nil
	}
	m.sel = i
	return m.loadPreview()
}

// advance moves the selection to the next row.
func (m *model) advance() tea.Cmd { return m.selectRow(m.sel + 1) }

// markReviewed is an unguarded mark-reviewed call.
func markReviewed(ctx context.Context, c client.Client, id int64) tea.Cmd {
	return func() tea.Msg {
		return reviewedMsg{id: id, err: c.MarkReviewed(ctx, id)}
	}
}

// reviewedMsg is the answer to a mark-reviewed.
type reviewedMsg struct {
	id  int64
	err error
}

func (m *model) markedReviewed(msg reviewedMsg) {
	if msg.err != nil {
		m.setError(msg.err)
		return
	}
	m.noteReviewed(msg.id)
}

// noteReviewed records a successful mark-reviewed in the row and the preview:
// it moves neither Version, so a revalidation would not see it.
func (m *model) noteReviewed(id int64) {
	m.reviewed[id] = true
	at := &api.Time{Time: m.now()}
	m.updateRow(id, func(r *row) { r.ReviewedAt = at })
	if i, ok := m.cache[id]; ok {
		i.ReviewedAt = at
		m.cache[id] = i
	}
}
