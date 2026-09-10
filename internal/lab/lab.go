// Package lab is what the lab's client and its server agree on: the world's
// size, the art and its sheets, the three skills and their cooldowns, the
// bouncers and the bounce that moves them, the floor and the edge of the
// world.
//
// Nothing here touches a socket or the browser, so it builds and tests on the
// host and compiles into the browser module unchanged. The server runs these
// rules and the client draws their result; the solo scene runs them on its
// own engine, so the sprites the lab measures move by the same code either
// way.
package lab

import (
	"math/rand/v2"
	"slices"

	"github.com/andygeiss/wisp-engine"
)

// The bouncers and the skills' numbers. Speeds are pixels per millisecond
// and times are milliseconds, like everything in the engine.
const (
	BounceSpeed  = 0.15
	DashDuration = 300.0
	DashFactor   = 4.0
	SpawnBatch   = 10
	SpawnSpread  = 64.0
	// StrikeDuration is how long the strike's animation plays, which is
	// every frame of it once.
	StrikeDuration = FramesStrike * FrameMS
)

// StateStrike marks a player mid-swing. It is the lab's own bit, above the
// engine's, and it is in the row mask: while it is set the engine draws the
// strike row and does not turn the player around.
const StateStrike = uint64(1 << 16)

// Skill is one of the three things a player can do besides move. They are
// game-jam-template's keys and cooldowns, so the day that game is networked
// its Q, E and R map one to one.
type Skill uint8

// The skills, in the order their bits and their cooldowns travel.
const (
	Strike Skill = iota
	Spawn
	Dash
	NumSkills
)

// Key is the key that fires s, the way the engine names keys.
func (s Skill) Key() string {
	switch s {
	case Strike:
		return "q"
	case Spawn:
		return "e"
	case Dash:
		return "r"
	}
	return ""
}

// String names s for a HUD.
func (s Skill) String() string {
	switch s {
	case Strike:
		return "strike"
	case Spawn:
		return "spawn"
	case Dash:
		return "dash"
	}
	return ""
}

// Cooldown is how long after firing s may fire again, in milliseconds.
func (s Skill) Cooldown() float64 {
	switch s {
	case Strike:
		return 1000
	case Spawn:
		return 3000
	case Dash:
		return 5000
	}
	return 0
}

// Setup gives e the lab's sheets, its eight-way facing, its pose rows and
// its world size. Both halves call it: the server so its players pick rows
// and its bouncers walk the right way, the client so it draws what the
// server describes.
func Setup(e *wisp.Engine) {
	e.Facing = NumDirections
	e.Sheets = []wisp.Sheet{ImageHero: HeroSheet(), ImageFox: FoxSheet()}
	e.RowMask = wisp.MaskPose | StateStrike
	e.RowForState = Rows()
	e.SetWorldSize(WorldW, WorldH)
}

// ClampToWorld keeps entity i inside the world, sprite and all. The engine
// moves an entity by its bits and stops at nothing; the edge of the world is
// the lab's rule, so the lab applies it after every tick.
func ClampToWorld(e *wisp.Engine, i int) {
	hw, hh := e.SpriteWidth[i]/2, e.SpriteHeight[i]/2
	e.X[i] = min(max(e.X[i], hw), WorldW-hw)
	e.Y[i] = min(max(e.Y[i], hh), WorldH-hh)
}

// Player is one player's share of the rules: their entity, the cooldown left
// on each skill, and the dash that is running. On the server there is one per
// connection and the cooldowns decide; a client only ever holds a copy of
// them, for its bars.
type Player struct {
	// Entity is the player's index in the engine.
	Entity int
	// Cooldown is how many milliseconds until each skill may fire again.
	Cooldown [NumSkills]float64

	e      *wisp.Engine
	dash   float64
	strike float64
}

// NewPlayer adds a player at the centre of the world, facing south the way
// a generated character is drawn first, and returns their rules.
func NewPlayer(e *wisp.Engine) *Player {
	i := e.Add(wisp.Sprite{
		Height: HeroH, Image: ImageHero, Row: Tag(HeroIdle, South),
		State: wisp.StateAnimated | wisp.StateAnimatedLoop | wisp.StateFaceDown |
			wisp.StateIdle | wisp.StateVisible,
		Width: HeroW, X: WorldW / 2, Y: WorldH / 2, Z: ZPlayer,
	})
	return &Player{Entity: i, e: e}
}

// Tick runs the cooldowns, the dash and the strike down by dt milliseconds
// and keeps the player inside the world. Call it once per tick, after the
// engine has moved the entity, so a dash ends on the tick its time runs out
// and a step past the edge is taken back the same tick.
func (p *Player) Tick(dt float64) {
	for s := range p.Cooldown {
		p.Cooldown[s] = max(p.Cooldown[s]-dt, 0)
	}
	if p.dash > 0 {
		p.dash -= dt
		if p.dash <= 0 {
			p.dash = 0
			p.e.SpeedFactor[p.Entity] = 1
		}
	}
	if p.strike > 0 {
		p.strike -= dt
		if p.strike <= 0 {
			p.strike = 0
			p.e.State[p.Entity] &^= StateStrike
		}
	}
	ClampToWorld(p.e, p.Entity)
}

// Ready reports whether s may fire now: its cooldown is at zero.
func (p *Player) Ready(s Skill) bool { return s < NumSkills && p.Cooldown[s] <= 0 }

// Fire runs s if it is ready, starts its cooldown, and reports whether it
// fired. A strike deletes every bouncer whose hit box overlaps the player's
// and returns them, and plays the swing for [StrikeDuration]; a spawn adds
// up to [SpawnBatch] bouncers around the player, fewer at the cap, and
// returns those; a dash makes the player [DashFactor] times as fast for
// [DashDuration] and returns nothing. A press on cooldown does nothing at
// all, whatever sent it.
func (p *Player) Fire(s Skill, b *Bouncers) (affected []int, fired bool) {
	if !p.Ready(s) {
		return nil, false
	}
	p.Cooldown[s] = s.Cooldown()
	switch s {
	case Strike:
		p.e.State[p.Entity] |= StateStrike
		p.strike = StrikeDuration
		return b.Struck(p.Entity), true
	case Spawn:
		x, y := p.e.X[p.Entity], p.e.Y[p.Entity]
		for range SpawnBatch {
			i := b.Add(x+(rand.Float64()-0.5)*SpawnSpread, y+(rand.Float64()-0.5)*SpawnSpread)
			if i < 0 {
				break
			}
			affected = append(affected, i)
		}
		return affected, true
	case Dash:
		p.e.SpeedFactor[p.Entity] = DashFactor
		p.dash = DashDuration
		return nil, true
	}
	return nil, false
}

// Bouncers is the lab's crowd: sprites that carry a velocity and turn around
// at the world's edges. They are what the lab spawns to measure a frame, and
// what a strike is for.
//
// A bouncer is moved from a tick, never from a frame, so the draw can blend
// it between two ticks: a sprite moved once per frame is already exactly
// where that frame wants it, and blending would drag it backwards.
type Bouncers struct {
	// Max is how many there may be at once; 0 means as many as you ask for.
	Max int

	e      *wisp.Engine
	idx    []int
	vx, vy []float64
}

// NewBouncers returns an empty crowd on e, capped at max.
func NewBouncers(e *wisp.Engine, max int) *Bouncers { return &Bouncers{Max: max, e: e} }

// Len is how many bouncers there are.
func (b *Bouncers) Len() int { return len(b.idx) }

// Add spawns one bouncer at (x, y) with a random velocity, a random layer, a
// random starting frame and a random opacity from 60 to 100 per cent — all of
// which make it cost something to draw, which is what it is for. It walks
// the way it is going. It returns the entity's index, or -1 at the cap.
func (b *Bouncers) Add(x, y float64) int {
	if b.Max > 0 && len(b.idx) >= b.Max {
		return -1
	}
	vx, vy := (rand.Float64()*2-1)*BounceSpeed, (rand.Float64()*2-1)*BounceSpeed
	i := b.e.Add(wisp.Sprite{
		Alpha:  0.6 + rand.Float64()*0.4,
		Height: FoxH, Image: ImageFox, Row: Tag(FoxWalk, DirectionOf(vx, vy)),
		State: wisp.StateAnimated | wisp.StateAnimatedLoop | wisp.StateVisible,
		Width: FoxW, X: x, Y: y, Z: rand.IntN(3),
	})
	b.e.FrameOffset[i] = rand.IntN(FramesWalk)
	b.idx = append(b.idx, i)
	b.vx = append(b.vx, vx)
	b.vy = append(b.vy, vy)
	return i
}

// Velocity returns bouncer i's velocity in pixels per millisecond, or zeros
// when i is not a bouncer.
func (b *Bouncers) Velocity(i int) (vx, vy float64) {
	if n := slices.Index(b.idx, i); n >= 0 {
		return b.vx[n], b.vy[n]
	}
	return 0, 0
}

// SetVelocity launches bouncer i at (vx, vy) pixels per millisecond, and
// turns its sprite to walk that way.
func (b *Bouncers) SetVelocity(i int, vx, vy float64) {
	if n := slices.Index(b.idx, i); n >= 0 {
		b.vx[n], b.vy[n] = vx, vy
		b.e.ImageRow[i] = Tag(FoxWalk, DirectionOf(vx, vy))
	}
}

// Remove deletes bouncer i and reports whether there was one.
func (b *Bouncers) Remove(i int) bool {
	n := slices.Index(b.idx, i)
	if n < 0 {
		return false
	}
	b.remove(n)
	return true
}

// RemoveLast deletes the newest bouncer and returns its index, or -1 when
// there are none. Removing newest first is what lets a spawner's count go
// back down the way it went up.
func (b *Bouncers) RemoveLast() int {
	n := len(b.idx) - 1
	if n < 0 {
		return -1
	}
	i := b.idx[n]
	b.remove(n)
	return i
}

// remove deletes the bouncer at position n of the list, keeping the order of
// the rest so RemoveLast still means newest.
func (b *Bouncers) remove(n int) {
	b.e.Delete(b.idx[n])
	b.idx = slices.Delete(b.idx, n, n+1)
	b.vx = slices.Delete(b.vx, n, n+1)
	b.vy = slices.Delete(b.vy, n, n+1)
}

// Struck deletes every bouncer whose hit box overlaps entity by's — on this
// engine's positions, which on the server are the only ones that count — and
// returns their indices, so a server can say which ones went.
func (b *Bouncers) Struck(by int) (hit []int) {
	for n := 0; n < len(b.idx); {
		if i := b.idx[n]; b.e.HasCollision(by, i) {
			hit = append(hit, i)
			b.remove(n)
			continue
		}
		n++
	}
	return hit
}

// Simulate moves every bouncer by dt milliseconds and turns it around at the
// world's edges — the sprite too, so it keeps walking the way it goes. Call
// it from [wisp.Engine.Simulate], so it runs on the tick.
func (b *Bouncers) Simulate(dt float64) {
	e := b.e
	for n, i := range b.idx {
		e.X[i] += b.vx[n] * dt
		e.Y[i] += b.vy[n] * dt
		turned := false
		if e.X[i] < 0 || e.X[i] > WorldW {
			b.vx[n] = -b.vx[n]
			turned = true
		}
		if e.Y[i] < 0 || e.Y[i] > WorldH {
			b.vy[n] = -b.vy[n]
			turned = true
		}
		if turned {
			// The row alone: the frame keeps counting, so the stride does
			// not restart at every wall.
			e.ImageRow[i] = Tag(FoxWalk, DirectionOf(b.vx[n], b.vy[n]))
		}
	}
}
