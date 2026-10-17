// Input intents: the players' intentions, never game state.
// The contract is settled in
// tasks/closed/define-input-intent-contract.md.
package game

import "errors"

// Role identifies which set of controls an intent drives. The server maps
// each round's rolled player to a role (`Sim.RoleOf`): the pilot's controls
// drive the ship, the gunner's drive the weapons.
type Role string

const (
	RolePilot  Role = "pilot"
	RoleGunner Role = "gunner"
)

// Intent is a player's latest intention. Only the fields for the
// player's role are meaningful: Rotate, Steer, and Thrust for the pilot,
// Fire and Aim for the gunner.
type Intent struct {
	// Seq is the client's monotonic counter; it orders posts that the
	// network may reorder or duplicate.
	Seq uint64
	// Rotate is the pilot's turn axis: -1, 0, or +1. It is what the
	// keyboard sends, and it turns the ship when no Steer is posted.
	Rotate int
	// Steer is the pilot's stick direction in radians, an absolute
	// world angle: the heading the stick points at, 0 along +X. It is
	// optional; with one the ship turns toward it and rests on it, and
	// it wins over Rotate when both are present.
	Steer *float64
	// Thrust is the pilot's thrust button.
	Thrust bool
	// Fire is the gunner's fire button.
	Fire bool
	// Aim is the gunner's turret direction in radians, an absolute
	// world angle. It is optional: without one the turret follows the
	// ship's heading, which is also where a stale intent leaves it.
	Aim *float64
}

// neutral is the intent applied when a role's latest intent has gone
// stale: every continuous action stops.
var neutral = Intent{}

// Errors reported by SubmitIntent. ErrStopped is declared with the loop.
var (
	// ErrRateLimited means the role exceeded the contract's coarse cap.
	ErrRateLimited = errors.New("intent rate exceeded")
	// ErrUnknownRole means the role is not one the sim understands.
	ErrUnknownRole = errors.New("unknown role")
)
