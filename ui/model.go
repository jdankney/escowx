// Package ui is the full-screen Bubble Tea dashboard.
package ui

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/j4mes/escowx/api"
	"github.com/j4mes/escowx/config"
	"github.com/j4mes/escowx/demo"
	"github.com/j4mes/escowx/modes"
	"github.com/j4mes/escowx/tracker"
	"github.com/j4mes/escowx/wx"
)

const (
	pointRetry   = 30 * time.Second
	fetchTimeout = 90 * time.Second // per fetch, including retries
	frameTick    = 500 * time.Millisecond
)

type pane int

const (
	paneLocations pane = iota
	paneAlerts
)

type Model struct {
	cfg     *config.Config
	client  *api.Client
	tracker *tracker.Tracker
	ctx     context.Context
	cancel  context.CancelFunc

	locs []*wx.Location
	now  time.Time

	width, height int
	gridCols      int // columns in the surrounding-locations grid, from the last layout

	pane     pane
	sel      int // selected location
	focus    int // location shown in the large panel
	alertSel int
	alertOff int
	alerts   []wx.AlertEntry // merged, sorted; rebuilt on every change

	help     bool
	expanded bool
	vp       viewport.Model

	demo      bool
	demoModes map[int]modes.Mode

	flash bool

	// refresh generations: bumping one orphans the pending tick chain so a
	// forced refresh restarts the interval instead of doubling up.
	alertGen, wxGen int
	alertBusy       []bool
	wxBusy          []bool
	pointBusy       []bool
}

// New builds the dashboard model.
func New(cfg *config.Config, client *api.Client, tr *tracker.Tracker, demoMode bool) *Model {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Model{
		cfg: cfg, client: client, tracker: tr,
		ctx: ctx, cancel: cancel,
		now:       time.Now(),
		gridCols:  2,
		demo:      demoMode,
		alertBusy: make([]bool, len(cfg.Locations)),
		wxBusy:    make([]bool, len(cfg.Locations)),
		pointBusy: make([]bool, len(cfg.Locations)),
	}
	for _, l := range cfg.Locations {
		m.locs = append(m.locs, &wx.Location{Cfg: l})
	}
	if demoMode {
		m.demoModes = demo.Initial(len(m.locs))
		m.applyDemo()
	}
	return m
}

// ── messages ──────────────────────────────────────────────────────────────

type pointMsg struct {
	i   int
	pt  *api.Point
	err error
}
type retryPointMsg struct{ i int }
type alertsMsg struct {
	i      int
	alerts []api.Alert
	err    error
}
type wxMsg struct {
	i   int
	res wx.WeatherResult
}
type frameMsg time.Time
type alertTickMsg struct{ gen int }
type wxTickMsg struct{ gen int }

// ── commands ──────────────────────────────────────────────────────────────

func (m *Model) resolvePoint(i int) tea.Cmd {
	if m.pointBusy[i] {
		return nil
	}
	m.pointBusy[i] = true
	loc := m.locs[i].Cfg
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.ctx, fetchTimeout)
		defer cancel()
		pt, err := m.client.ResolvePoint(ctx, loc.Lat, loc.Lon)
		return pointMsg{i, pt, err}
	}
}

func (m *Model) fetchAlerts(i int) tea.Cmd {
	if m.alertBusy[i] {
		return nil
	}
	m.alertBusy[i] = true
	loc := m.locs[i].Cfg
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.ctx, fetchTimeout)
		defer cancel()
		a, err := m.client.ActiveAlerts(ctx, loc.Lat, loc.Lon)
		return alertsMsg{i, a, err}
	}
}

func (m *Model) fetchWeather(i int) tea.Cmd {
	pt := m.locs[i].Point
	if pt == nil || m.wxBusy[i] {
		return nil
	}
	m.wxBusy[i] = true
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.ctx, fetchTimeout)
		defer cancel()
		return wxMsg{i, wx.FetchWeather(ctx, m.client, pt)}
	}
}

func frame() tea.Cmd {
	return tea.Tick(frameTick, func(t time.Time) tea.Msg { return frameMsg(t) })
}

func (m *Model) scheduleAlerts() tea.Cmd {
	gen := m.alertGen
	return tea.Tick(m.cfg.Refresh.Alerts, func(time.Time) tea.Msg { return alertTickMsg{gen} })
}

func (m *Model) scheduleWx() tea.Cmd {
	gen := m.wxGen
	return tea.Tick(m.cfg.Refresh.Forecast, func(time.Time) tea.Msg { return wxTickMsg{gen} })
}

// refreshAll fetches everything now and restarts both refresh intervals.
func (m *Model) refreshAll() tea.Cmd {
	m.alertGen++
	m.wxGen++
	cmds := []tea.Cmd{m.scheduleAlerts(), m.scheduleWx()}
	for i, l := range m.locs {
		cmds = append(cmds, m.fetchAlerts(i))
		if l.Point == nil {
			cmds = append(cmds, m.resolvePoint(i))
		} else {
			cmds = append(cmds, m.fetchWeather(i))
		}
	}
	return tea.Batch(cmds...)
}

func (m *Model) Init() tea.Cmd {
	cmds := []tea.Cmd{frame(), m.scheduleAlerts(), m.scheduleWx()}
	for i := range m.locs {
		// Alerts only need lat/lon, so they don't wait on point resolution.
		cmds = append(cmds, m.resolvePoint(i), m.fetchAlerts(i))
	}
	return tea.Batch(cmds...)
}

// ── update ────────────────────────────────────────────────────────────────

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.layoutViewport()
		return m, nil

	case tea.KeyMsg:
		return m, m.handleKey(msg)

	case frameMsg:
		m.now = time.Time(msg)
		m.flash = !m.flash
		m.reclassify() // drops alerts as they expire
		return m, frame()

	case pointMsg:
		m.pointBusy[msg.i] = false
		l := m.locs[msg.i]
		if msg.err != nil {
			l.PointErr = msg.err
			return m, tea.Tick(pointRetry, func(time.Time) tea.Msg { return retryPointMsg{msg.i} })
		}
		l.Point, l.PointErr = msg.pt, nil
		return m, m.fetchWeather(msg.i)

	case retryPointMsg:
		if m.locs[msg.i].Point == nil {
			return m, m.resolvePoint(msg.i)
		}
		return m, nil

	case alertsMsg:
		m.alertBusy[msg.i] = false
		m.locs[msg.i].ApplyAlerts(msg.alerts, msg.err, time.Now())
		m.reclassify()
		return m, nil

	case wxMsg:
		m.wxBusy[msg.i] = false
		m.locs[msg.i].ApplyWeather(msg.res, time.Now())
		return m, nil

	case alertTickMsg:
		if msg.gen != m.alertGen {
			return m, nil
		}
		cmds := []tea.Cmd{m.scheduleAlerts()}
		for i := range m.locs {
			cmds = append(cmds, m.fetchAlerts(i))
		}
		return m, tea.Batch(cmds...)

	case wxTickMsg:
		if msg.gen != m.wxGen {
			return m, nil
		}
		cmds := []tea.Cmd{m.scheduleWx()}
		for i := range m.locs {
			cmds = append(cmds, m.fetchWeather(i))
		}
		return m, tea.Batch(cmds...)
	}
	if m.expanded {
		var cmd tea.Cmd
		m.vp, cmd = m.vp.Update(msg)
		return m, cmd
	}
	return m, nil
}

// reclassify recomputes every location's mode, reports transitions to the
// tracker, and rebuilds the merged alert list.
func (m *Model) reclassify() {
	for _, l := range m.locs {
		l.Reclassify(m.now)
		if m.tracker != nil {
			m.tracker.Observe(l.Cfg.Name, l.Mode, l.Cause)
		}
	}
	m.alerts = wx.MergeAlerts(m.locs, m.now)
	if m.alertSel >= len(m.alerts) {
		m.alertSel = max(0, len(m.alerts)-1)
	}
}

func (m *Model) applyDemo() {
	for i, l := range m.locs {
		l.DemoAlerts = demo.Alert(m.demoModes[i], l.Cfg.Name, m.now)
	}
	m.reclassify()
}

// ── keys ──────────────────────────────────────────────────────────────────

func (m *Model) handleKey(k tea.KeyMsg) tea.Cmd {
	key := k.String()
	if key == "ctrl+c" {
		m.cancel()
		return tea.Quit
	}
	if m.help {
		if key == "q" {
			m.cancel()
			return tea.Quit
		}
		m.help = false
		return nil
	}
	if m.expanded {
		switch key {
		case "esc", "enter", "backspace":
			m.expanded = false
		case "q":
			m.cancel()
			return tea.Quit
		default:
			var cmd tea.Cmd
			m.vp, cmd = m.vp.Update(k)
			return cmd
		}
		return nil
	}

	switch key {
	case "q":
		m.cancel()
		return tea.Quit
	case "?":
		m.help = true
	case "r":
		return m.refreshAll()
	case "esc":
		m.focus = 0
		m.pane = paneLocations
	case "a":
		if len(m.alerts) > 0 {
			m.pane = paneAlerts
		}
	case "tab":
		m.cycle(1)
	case "shift+tab":
		m.cycle(-1)
	case "left", "h":
		m.moveH(-1)
	case "right", "l":
		m.moveH(1)
	case "up", "k":
		m.moveV(-1)
	case "down", "j":
		m.moveV(1)
	case "pgup":
		if m.pane == paneAlerts {
			m.alertSel = max(0, m.alertSel-5)
		}
	case "pgdown":
		if m.pane == paneAlerts {
			m.alertSel = min(len(m.alerts)-1, m.alertSel+5)
		}
	case "enter", " ":
		if m.pane == paneAlerts {
			if len(m.alerts) > 0 {
				m.expanded = true
				m.layoutViewport()
				m.vp.SetContent(m.alertDetail(m.alerts[m.alertSel]))
				m.vp.GotoTop()
			}
		} else if m.focus == m.sel && m.sel != 0 {
			m.focus = 0 // enter again on the focused location returns to primary
		} else {
			m.focus = m.sel
		}
	case "m", "M":
		if !m.demo {
			return nil
		}
		if key == "m" {
			m.demoModes[m.sel] = (m.demoModes[m.sel] + 1) % 4
		} else {
			// Cycle every location together, based on the selected one.
			next := (m.demoModes[m.sel] + 1) % 4
			for i := range m.locs {
				m.demoModes[i] = next
			}
		}
		m.applyDemo()
	}
	return nil
}

// cycle moves through every location, then the alerts pane, and around.
func (m *Model) cycle(d int) {
	n := len(m.locs)
	stops := n
	if len(m.alerts) > 0 {
		stops++
	}
	cur := m.sel
	if m.pane == paneAlerts {
		cur = n
	}
	next := ((cur+d)%stops + stops) % stops
	if next == n {
		m.pane = paneAlerts
	} else {
		m.pane, m.sel = paneLocations, next
	}
}

// moveH moves left/right: between the large panel and the grid, and along
// grid rows.
func (m *Model) moveH(d int) {
	if m.pane != paneLocations {
		return
	}
	n, cols := len(m.locs), max(1, m.gridCols)
	if m.sel == 0 {
		if d > 0 && n > 1 {
			m.sel = 1
		}
		return
	}
	col := (m.sel - 1) % cols
	switch {
	case d < 0 && col == 0:
		m.sel = 0
	case d < 0:
		m.sel--
	case d > 0 && col < cols-1 && m.sel+1 < n:
		m.sel++
	}
}

// moveV moves up/down within the grid, falling into / out of the alerts
// pane at the bottom.
func (m *Model) moveV(d int) {
	n, cols := len(m.locs), max(1, m.gridCols)
	if m.pane == paneAlerts {
		if d < 0 && m.alertSel == 0 {
			m.pane = paneLocations
			return
		}
		m.alertSel = max(0, min(len(m.alerts)-1, m.alertSel+d))
		return
	}
	if m.sel == 0 {
		if d > 0 && len(m.alerts) > 0 {
			m.pane = paneAlerts
		}
		return
	}
	next := m.sel + d*cols
	switch {
	case next >= 1 && next < n:
		m.sel = next
	case d > 0 && len(m.alerts) > 0:
		m.pane = paneAlerts
	}
}

func (m *Model) layoutViewport() {
	w, h := max(20, m.width-8), max(5, m.height-6)
	if m.vp.Width == 0 {
		m.vp = viewport.New(w, h)
	} else {
		m.vp.Width, m.vp.Height = w, h
	}
	if m.expanded && len(m.alerts) > 0 {
		m.vp.SetContent(m.alertDetail(m.alerts[m.alertSel]))
	}
}

// Shutdown cancels in-flight requests.
func (m *Model) Shutdown() { m.cancel() }

// statusLine summarises locations that are failing to load.
func (m *Model) statusLine() string {
	for _, l := range m.locs {
		if l.Point == nil && l.PointErr != nil {
			return fmt.Sprintf("%s: location lookup failed (%v), retrying", l.Cfg.Name, l.PointErr)
		}
	}
	return ""
}
