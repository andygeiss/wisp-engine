package wisp

import "strings"

// storageKey is where the browser remembers a tuned set between reloads.
const storageKey = "wisp.settings"

// saveDelay is how long the menu waits after the last change before writing.
// Nudging a knob with a held arrow key would otherwise write on every frame.
const saveDelay = 500.0

// rowKind says whether a menu row is a group heading or one knob.
type rowKind uint8

const (
	rowGroup rowKind = iota
	rowKnob
)

// row is one line of the menu: a group heading, or a knob inside an open one.
type row struct {
	Group string
	Index int // into the knob table, for rowKnob
	Kind  rowKind
}

// menu is the tuning overlay. It edits the engine's own [Settings] while the
// game runs, because a camera and a hit stop can only be judged in motion.
//
// While it is open it takes the keyboard: every [Input.Down] and
// [Input.JustPressed] a game asks about answers false, so the player cannot
// steer and the game's own logic keeps running untouched.
type menu struct {
	defaults Settings
	dirty    float64 // milliseconds since the last change, or -1 for none
	open     bool
	opened   map[string]bool
	rows     []row
	sel      int
	status   string
	statusAt float64
	table    []knob
	top      int // the first row drawn, so a long list scrolls
}

// newMenu builds the knob table once. It hangs off the engine rather than a
// package variable, so the package holds no mutable state.
func newMenu() menu {
	return menu{
		defaults: Defaults(),
		dirty:    -1,
		opened:   make(map[string]bool),
		table:    knobTable(),
	}
}

// Menu reports whether the tuning menu is open. A game that draws its own
// overlay can use it to get out of the way.
func (e *Engine) Menu() bool { return e.menu.open }

// step reads the menu's keys and runs its save timer. It runs before the
// game's update, so a key the menu consumes never reaches the game.
func (m *menu) step(e *Engine, elapsed float64) {
	if e.menuKey == KeyNone {
		return
	}
	if e.Input.pressedRaw(e.menuKey) {
		m.toggle(e)
	}
	if m.status != "" {
		m.statusAt += elapsed
		if m.statusAt > 2000 {
			m.status, m.statusAt = "", 0
		}
	}
	if m.dirty >= 0 {
		m.dirty += elapsed
		if m.dirty >= saveDelay {
			m.save(e)
		}
	}
	if !m.open {
		return
	}
	m.edit(e)
}

// toggle opens or closes the menu, locking or freeing the keyboard with it.
func (m *menu) toggle(e *Engine) {
	m.open = !m.open
	e.Input.locked = m.open
	if m.open {
		m.layout()
	}
}

// edit is the whole keyboard of an open menu.
func (m *menu) edit(e *Engine) {
	in := &e.Input
	switch {
	case in.pressedRaw("Escape"):
		m.toggle(e)
		return
	case in.pressedRaw("ArrowUp"):
		m.move(-1)
	case in.pressedRaw("ArrowDown"):
		m.move(1)
	case in.pressedRaw("Enter"):
		m.enter(e)
	case in.pressedRaw("ArrowLeft"):
		m.nudge(e, -1)
	case in.pressedRaw("ArrowRight"):
		m.nudge(e, 1)
	case in.pressedRaw("Backspace"):
		m.reset(e)
	case in.pressedRaw("c"):
		e.rt.copyText(e.Settings.GoLiteral())
		m.say("copied as Go")
	case in.pressedRaw("t"):
		e.Impact(e.Feel.Medium)
	}
}

// enter opens or closes a group, or flips a bool.
func (m *menu) enter(e *Engine) {
	if len(m.rows) == 0 {
		return
	}
	r := m.rows[m.sel]
	if r.Kind == rowGroup {
		m.opened[r.Group] = !m.opened[r.Group]
		m.layout()
		return
	}
	k := m.table[r.Index]
	if k.Kind != knobBool {
		return
	}
	if k.Get(&e.Settings) != 0 {
		k.Set(&e.Settings, 0)
	} else {
		k.Set(&e.Settings, 1)
	}
	m.changed()
}

// layout rebuilds the visible rows. Groups start closed, so the whole list
// fits on a 360 px canvas and opening one is a deliberate act.
func (m *menu) layout() {
	m.rows = m.rows[:0]
	for _, g := range groups() {
		// Feel holds only sub-structs, so it has no knobs of its own and no
		// heading to open. The literal still nests it; the menu does not.
		var own []int
		for i, k := range m.table {
			if k.Group == g.Path {
				own = append(own, i)
			}
		}
		if len(own) == 0 {
			continue
		}
		m.rows = append(m.rows, row{Group: g.Path, Kind: rowGroup})
		if !m.opened[g.Path] {
			continue
		}
		for _, i := range own {
			m.rows = append(m.rows, row{Group: g.Path, Index: i, Kind: rowKnob})
		}
	}
	m.sel = min(m.sel, max(len(m.rows)-1, 0))
}

// move walks the selection and scrolls to keep it visible.
func (m *menu) move(by int) {
	if len(m.rows) == 0 {
		return
	}
	m.sel = (m.sel + by + len(m.rows)) % len(m.rows)
	m.top = min(m.top, m.sel)
	if m.sel >= m.top+menuRows {
		m.top = m.sel - menuRows + 1
	}
}

// nudge steps the selected knob. Shift takes ten steps and Ctrl a tenth of
// one, which is the difference between finding the right order of magnitude
// and settling on a number.
func (m *menu) nudge(e *Engine, dir float64) {
	if len(m.rows) == 0 || m.rows[m.sel].Kind != rowKnob {
		return
	}
	k := m.table[m.rows[m.sel].Index]
	step := k.Step
	switch {
	case e.Input.Shift:
		step *= 10
	case e.Input.Ctrl:
		step /= 10
	}
	k.Set(&e.Settings, k.clampTo(k.Get(&e.Settings)+dir*step))
	m.changed()
}

// reset puts the selected knob back to its default, or with shift the whole
// set.
func (m *menu) reset(e *Engine) {
	if e.Input.Shift {
		e.Settings = m.defaults
		m.say("reset everything")
		m.changed()
		return
	}
	if len(m.rows) == 0 || m.rows[m.sel].Kind != rowKnob {
		return
	}
	k := m.table[m.rows[m.sel].Index]
	k.Set(&e.Settings, k.Get(&m.defaults))
	m.say("reset " + k.Field)
	m.changed()
}

// changed restarts the save timer.
func (m *menu) changed() { m.dirty = 0 }

// say puts a line in the footer for a couple of seconds.
func (m *menu) say(s string) { m.status, m.statusAt = s, 0 }

// load reads a tuned set the browser remembered. A game with no menu never
// calls it, so a shipped build always starts on its compiled-in settings.
func (m *menu) load(e *Engine) {
	if e.menuKey == KeyNone {
		return
	}
	if text := e.rt.storeGet(storageKey); text != "" {
		_ = e.Settings.UnmarshalText([]byte(text))
	}
}

// save writes the tuned set back.
func (m *menu) save(e *Engine) {
	m.dirty = -1
	if e.menuKey == KeyNone {
		return
	}
	text, err := e.Settings.MarshalText()
	if err != nil {
		return
	}
	e.rt.storeSet(storageKey, string(text))
	m.say("saved")
}

// The menu's own measurements, in canvas pixels.
const (
	menuFont    = "12px ui-monospace, monospace"
	menuPad     = 8.0
	menuRowH    = 14.0
	menuRows    = 18
	menuWidth   = 232.0
	menuMarkerX = 6.0
)

// draw paints the menu over the running scene. It uses the two primitives the
// engine already has, so it needs nothing from the page and survives
// fullscreen.
func (m *menu) draw(e *Engine) {
	if !m.open {
		return
	}
	const (
		dim      = "rgba(255, 255, 255, 0.55)"
		ground   = "rgba(18, 20, 26, 0.94)"
		border   = "rgba(255, 255, 255, 0.25)"
		selected = "yellow"
		plain    = "white"
	)

	x := e.Width - menuWidth
	h := float64(menuRows+4)*menuRowH + menuPad*2
	y := (e.Height - h) / 2

	e.Rect(x-2, y-2, menuWidth+2, h+4, border)
	e.Rect(x, y, menuWidth, h, ground)

	ty := y + menuPad + menuRowH/2
	e.Text(x+menuPad, ty, "Tune  ·  M closes", plain, menuFont, "left")
	ty += menuRowH * 1.5

	last := min(m.top+menuRows, len(m.rows))
	for i := m.top; i < last; i++ {
		r := m.rows[i]
		color := dim
		if i == m.sel {
			color = selected
			e.Text(x+menuMarkerX, ty, "▶", selected, menuFont, "left")
		}
		if r.Kind == rowGroup {
			mark := "+"
			if m.opened[r.Group] {
				mark = "-"
			}
			e.Text(x+menuPad+8, ty, mark+" "+r.Group, color, menuFont, "left")
			ty += menuRowH
			continue
		}
		k := m.table[r.Index]
		e.Text(x+menuPad+18, ty, k.Field, color, menuFont, "left")
		e.Text(x+menuWidth-menuPad, ty, k.text(&e.Settings), color, menuFont, "right")
		ty += menuRowH
	}

	ty = y + h - menuPad - menuRowH
	e.Text(x+menuPad, ty, m.footer(e), dim, menuFont, "left")
	e.Text(x+menuPad, ty-menuRowH, m.hint(), dim, menuFont, "left")
}

// footer says what just happened, or what the selected knob's range is.
func (m *menu) footer(e *Engine) string {
	if m.status != "" {
		return m.status
	}
	if len(m.rows) == 0 || m.rows[m.sel].Kind != rowKnob {
		return "Enter opens a group"
	}
	k := m.table[m.rows[m.sel].Index]
	if k.Kind == knobBool {
		return "Enter toggles"
	}
	var b strings.Builder
	b.WriteString(trimFloat(k.Min))
	b.WriteString(" .. ")
	b.WriteString(trimFloat(k.Max))
	b.WriteString("  step ")
	b.WriteString(trimFloat(k.Step))
	return b.String()
}

// hint is the one line of key help the menu always shows.
func (m *menu) hint() string { return "←→ adjust  ⇧ x10  C copy  T test" }
