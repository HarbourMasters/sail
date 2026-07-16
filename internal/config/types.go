// Package config holds the user's chat command, channel point redeem, and
// game-hook bindings, persisted as JSON in the OS user config directory.
package config

import "github.com/HarbourMasters/Sail/internal/sail"

// DefaultPort is the Sail TCP server port.
const DefaultPort = 43384

// StepKind is the discriminator for a Step.
type StepKind string

const (
	StepKindAction  StepKind = "action"  // action.apply
	StepKindRemove  StepKind = "remove"  // action.remove
	StepKindCommand StepKind = "command" // raw console command
	StepKindChat    StepKind = "chat"    // post a message to Twitch chat
)

// Step is one thing to do when a binding fires: start (or re-trigger) an
// action, cancel a running one, run a console command, or post to Twitch chat
// as the logged-in user. Only the fields relevant to Kind are set.
//
// There's no "re-run after N seconds" field: a timed action carries its own
// Duration and the game ends it (via action.ended); use a StepKindRemove step
// to cancel one early.
type Step struct {
	Kind     StepKind       `json:"kind"`
	Name     string         `json:"name,omitempty"`     // action/remove: the action's name
	Params   map[string]any `json:"params,omitempty"`   // action: values for the action's param schema
	Duration *uint32        `json:"duration,omitempty"` // action: override its defaultDuration, in frames (20/sec)
	Lifetime sail.Lifetime  `json:"lifetime,omitempty"` // action: "session" (default) or "save"
	Command  string         `json:"command,omitempty"`  // command: the raw string to run
	Message  string         `json:"message,omitempty"`  // chat: the message text to post
}

// Binding is what a chat command or redemption runs: each Step fires
// immediately, in order. Script, when non-empty, is user JavaScript that
// produces the steps instead of the static Steps — for logic templates can't
// express; it wins when both are present (see App.fireBinding).
type Binding struct {
	Steps           []Step  `json:"steps"`
	Script          string  `json:"script,omitempty"`
	CooldownSeconds float64 `json:"cooldownSeconds,omitempty"`
}

// Command is a chat-triggered binding, e.g. "!kick" or "!rave". Chat text
// after the trigger word is available to action params (and command
// strings) as {{0}}, {{1}}, etc.
type Command struct {
	Trigger string  `json:"trigger"`
	Binding Binding `json:"binding"`
}

// Redeem binds a channel point reward (by ID) to a Binding. The reward itself
// (name, cost, input) lives on Twitch; RewardTitle is a display cache for when
// Twitch is unreachable.
type Redeem struct {
	RewardID    string  `json:"rewardId"`
	RewardTitle string  `json:"rewardTitle"`
	Binding     Binding `json:"binding"`
}

// HookBinding fires a Binding on a game hook (sail.HookName) — an in-game
// event — instead of anything from Twitch. IDFilter, when set, narrows to a
// single scene/item/actor id (only for filterable hooks); nil fires on every
// occurrence. A hook can carry several bindings (all matching ones fire), so
// identity is a generated ID, not the hook name.
type HookBinding struct {
	ID       string  `json:"id"`
	HookName string  `json:"hookName"`
	IDFilter *int    `json:"idFilter,omitempty"`
	Binding  Binding `json:"binding"`
}

// Config is the full set of user configuration persisted to disk.
type Config struct {
	Port     int           `json:"port"`
	Commands []Command     `json:"commands"`
	Redeems  []Redeem      `json:"redeems"`
	Hooks    []HookBinding `json:"hooks"`
}

// Default is the configuration a fresh install starts with.
func Default() Config {
	return Config{Port: DefaultPort}
}
