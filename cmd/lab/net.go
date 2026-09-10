//go:build js && wasm

// Network mode: the server runs the world and this half draws it. The engine's
// Simulate is the replica's Tick, so a snapshot is applied on the tick and the
// draw blends between two of them; the update given to Run reads the keys and
// sends what the player is trying to do, and nothing else.

package main

import (
	"github.com/andygeiss/wisp-engine"
	"github.com/andygeiss/wisp-engine/internal/lab"
	"github.com/andygeiss/wisp-engine/internal/replica"
	"github.com/andygeiss/wisp-engine/internal/wire"
)

const (
	// intentEvery is how often an intent is sent when nothing has changed,
	// in milliseconds. TCP loses nothing, so it is not a repeat; it is the
	// heartbeat the server's read deadline leans on.
	intentEvery = 500.0
	// pingEvery is how often the round trip is measured.
	pingEvery = 1000.0
	// rateEvery is how often the incoming byte rate is settled.
	rateEvery = 1000.0
)

// netMode is the client's side of the wire: the replica that holds the
// world, the socket that feeds it, and the numbers the HUD prints.
type netMode struct {
	e    *wisp.Engine
	r    *replica.Replica
	sock *socket
	buf  []byte // one outgoing message, reused

	seq         uint16
	dx, dy      float64 // the axis last sent
	lastNow     float64
	sinceIntent float64
	sincePing   float64
	sinceRate   float64

	rtt           float64 // milliseconds, from the last Pong
	bytesIn       int     // since the rate was last settled
	kbps          float64
	snapshotBytes int // the last snapshot's size on the wire
	err           error
}

// runNet connects to the server behind the page and plays its world.
func runNet(e *wisp.Engine) {
	n := &netMode{e: e, r: replica.New(e)}
	// Nothing here is steered or posed by this engine: every actor is the
	// server's, and it arrives with its bits already set.
	e.InputTarget, e.RowMask = -1, 0
	// The floor and the props never cross the wire; every client builds
	// its own.
	lab.BuildFloor(e)
	lab.BuildProps(e)
	n.r.OnEvent = n.onEvent
	e.Simulate = func(float64) { n.r.Tick() }
	e.RenderUI = n.render
	n.sock = dialSocket(socketURL(), n.onOpen, n.onMessage)
	n.lastNow = now()
	e.Run(n.update)
}

// onOpen says hello, which is what the server waits for before it lets a
// socket into the world.
func (n *netMode) onOpen() { n.send(wire.Hello{Version: wire.Version}.Append(n.buf[:0])) }

// onMessage takes one message off the socket: a Pong is timed here and now,
// everything else is the replica's, in order.
func (n *netMode) onMessage(b []byte) {
	n.bytesIn += len(b)
	m, err := wire.Decode(b)
	if err != nil {
		n.err = err
		return
	}
	switch m := m.(type) {
	case wire.Pong:
		n.rtt = now() - float64(m.T)
	case wire.Snapshot:
		n.snapshotBytes = len(b)
		n.r.Push(m)
	default:
		n.r.Push(m)
	}
}

// onEvent plays the feel of a skill of your own when the server says it
// fired. Other people's skills are theirs to feel.
func (n *netMode) onEvent(ev wire.Event) {
	if n.r.Welcomed && ev.Slot == n.r.YouSlot {
		n.e.Impact(n.e.Feel.Light)
	}
}

// update runs once a frame: the server's tick rate goes over the knob, the
// camera finds you, and the keys become an intent — sent when it changes and
// every intentEvery regardless. Edges are read here rather than in Simulate
// because a frame can carry two ticks or none.
func (n *netMode) update(float64) {
	e := n.e
	t := now()
	elapsed := t - n.lastNow
	n.lastNow = t

	if n.r.Welcomed {
		// Every frame, not once: a saved menu may still hold another rate,
		// and the blend has to know how long a server tick is.
		e.Time.TickRate = float64(n.r.TickRate)
		if e.CamTarget < 0 {
			e.CamTarget = n.r.You()
		}
	}
	if n.err == nil && n.r.Err != nil {
		n.err = n.r.Err
	}

	switch {
	case e.Input.JustPressed("F3"), e.Input.JustPressed("h"):
		e.Debug.ShowMetrics = !e.Debug.ShowMetrics
	case e.Input.JustPressed("1"):
		e.Impact(e.Feel.Light)
	case e.Input.JustPressed("2"):
		e.Impact(e.Feel.Medium)
	case e.Input.JustPressed("3"):
		e.Impact(e.Feel.Heavy)
	}

	dx, dy := e.Input.MoveAxis()
	var skills uint8
	for s := range lab.NumSkills {
		if e.Input.JustPressed(s.Key()) {
			skills |= 1 << s
		}
	}
	n.sinceIntent += elapsed
	if skills != 0 || dx != n.dx || dy != n.dy || n.sinceIntent >= intentEvery {
		n.seq++
		if n.send(wire.Intent{Seq: n.seq, DX: int8(dx), DY: int8(dy), Skills: skills}.Append(n.buf[:0])) {
			n.dx, n.dy, n.sinceIntent = dx, dy, 0
		}
	}

	n.sincePing += elapsed
	if n.sincePing >= pingEvery {
		n.sincePing = 0
		n.send(wire.Ping{T: uint32(t)}.Append(n.buf[:0]))
	}
	n.sinceRate += elapsed
	if n.sinceRate >= rateEvery {
		n.kbps = float64(n.bytesIn) / 1024 / (n.sinceRate / 1000)
		n.bytesIn, n.sinceRate = 0, 0
	}
}

// send writes one encoded message and keeps its buffer for the next. The
// callers encode with the concrete type's Append, never through the
// wire.Message interface: a second type through that one call costs a TinyGo
// build eleven kilobytes of dispatch, measured.
func (n *netMode) send(b []byte) bool {
	n.buf = b
	return n.sock.send(b)
}

// render draws the HUD: the state of the connection, a marker over your own
// sprite, the three cooldown bars from the server's copy, and the net line,
// which is the lab doing its job again — saying what the wire costs.
func (n *netMode) render() {
	e := n.e
	const (
		red   = "rgb(255, 96, 96)"
		ready = "rgba(120, 220, 140, 0.9)"
		cool  = "rgba(255, 255, 255, 0.25)"
	)

	if !e.Input.Started {
		e.Text(e.Width/2, e.Height/2, "Click to start", "white", "24px system-ui, sans-serif", "center")
		return
	}

	switch {
	case n.sock.closed:
		e.Text(e.Width/2, e.Height/2, "disconnected ("+itoa(n.sock.code)+") — reload to play", red, "16px system-ui, sans-serif", "center")
	case n.err != nil:
		e.Text(e.Width/2, e.Height/2, n.err.Error(), red, "16px system-ui, sans-serif", "center")
	case !n.r.Welcomed:
		e.Text(e.Width/2, e.Height/2, "connecting...", dim, "16px system-ui, sans-serif", "center")
	}

	// You: a mark over the sprite, at the drawn position rather than the
	// simulated one, or it would trail the sprite by a tick.
	if you := n.r.You(); you >= 0 {
		x, y := e.DrawPos(you)
		sx, sy := x-e.CamX+e.CamShakeX, y-e.CamY+e.CamShakeY
		e.Rect(sx-3, sy-e.SpriteHeight[you]/2-8, 6, 3, "yellow")
	}

	// The cooldowns, bottom right: a full bar is a skill that is ready.
	const barW, barH, rowH = 72.0, 5.0, 14.0
	y := e.Height - 24 - rowH*float64(lab.NumSkills)
	for s := range lab.NumSkills {
		left := 0.0
		if int(s) < len(n.r.Cooldowns) {
			left = float64(n.r.Cooldowns[s])
		}
		label := string(s.Key()[0]&^0x20) + " " + s.String()
		e.Text(e.Width-8-barW-6, y+rowH/2, label, dim, font, "right")
		e.Rect(e.Width-8-barW, y+rowH/2-barH/2, barW, barH, cool)
		if frac := 1 - left/s.Cooldown(); frac > 0 {
			color := ready
			if left > 0 {
				color = dim
			}
			e.Rect(e.Width-8-barW, y+rowH/2-barH/2, barW*frac, barH, color)
		}
		y += rowH
	}

	// The net line, and the keys.
	line := "rtt " + ftoa(n.rtt) + " ms   in " + ftoa(n.kbps) + " KB/s   snapshot " + itoa(n.snapshotBytes) + " B" +
		"   buffer " + itoa(n.r.Depth()) + "   held " + itoa(n.r.Held) + "   ff " + itoa(n.r.FastForwards) +
		"   catch " + itoa(n.r.CatchUps) + "   tick " + itoa(int(n.r.LastTick))
	e.Text(8, e.Height-24, line, "white", font, "left")
	e.Text(8, e.Height-10,
		"Q strike   E spawn   R dash   1 2 3 impact   M tune   H metrics   F full   ?solo for the ramp",
		dim, font, "left")
}
