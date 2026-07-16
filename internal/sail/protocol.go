// Package sail implements the Sail wire protocol: a TCP server the game
// connects to, speaking NUL-delimited JSON packets. Shapes here mirror
// 2ship2harkinian's mm/2s2h/Network/Sail/Sail.cpp (under active development —
// re-check it before assuming a shape is current).
package sail

// PacketType is the discriminator for every packet on the wire.
type PacketType string

const (
	PacketTypeCommand      PacketType = "command"
	PacketTypeActionApply  PacketType = "action.apply"
	PacketTypeActionRemove PacketType = "action.remove"
	PacketTypeActionList   PacketType = "action.list"
	PacketTypeActionStatus PacketType = "action.status"
	PacketTypeActionEnded  PacketType = "action.ended" // incoming only, unsolicited
	PacketTypeHookList     PacketType = "hook.list"
	PacketTypeSubscribe    PacketType = "subscribe"
	PacketTypeUnsubscribe  PacketType = "unsubscribe"
	PacketTypeResult       PacketType = "result" // incoming only
	PacketTypeHook         PacketType = "hook"   // incoming only, unsolicited
)

// Outcome is the status word on every result (and, for a timed action, its
// later action.ended).
type Outcome string

const (
	OutcomeOK         Outcome = "ok"         // a query or command did what was asked
	OutcomeApplied    Outcome = "applied"    // the action happened (or a timed one started)
	OutcomeFinished   Outcome = "finished"   // a timed action's duration ran out
	OutcomeCancelled  Outcome = "cancelled"  // it was stopped, or dropped when a save loaded
	OutcomeExpired    Outcome = "expired"    // the game never became ready in time
	OutcomeImpossible Outcome = "impossible" // it can never apply as asked
	OutcomeInvalid    Outcome = "invalid"    // the message itself was wrong; Reason says how

	// OutcomeTimeout never arrives over the wire; it's synthesized locally on a
	// timeout.
	OutcomeTimeout Outcome = "timeout"
)

// Lifetime is which game-state boundary an applied action does not survive.
type Lifetime string

const (
	// LifetimeSession survives everything except its own duration, an explicit
	// RemoveAction, or the emitter ending it — the game's default when Lifetime
	// is empty.
	LifetimeSession Lifetime = "session"
	// LifetimeSave is dropped when a save is loaded (file select, map
	// select, or the opening).
	LifetimeSave Lifetime = "save"
)

// ParamType is the type of a single action parameter, as reported by
// action.list.
type ParamType string

const (
	ParamBool   ParamType = "bool"
	ParamInt    ParamType = "int"
	ParamFloat  ParamType = "float"
	ParamString ParamType = "string"
)

// ParamSpec describes one parameter an action accepts, as reported by
// action.list. Default/Min/Max are only present when the game sent them.
type ParamSpec struct {
	Name     string    `json:"name"`
	Type     ParamType `json:"type"`
	Required bool      `json:"required"`
	Default  any       `json:"default,omitempty"`
	Min      *float64  `json:"min,omitempty"`
	Max      *float64  `json:"max,omitempty"`
}

// ActionInfo describes one action, as reported by action.list. Stacking is
// "queue" (wait for the running one) or "refresh" (restart its timer); Valence
// ("positive"/"negative"/"neutral") is UI-only metadata for a remote.
type ActionInfo struct {
	Name            string      `json:"name"`
	DisplayName     string      `json:"displayName"`
	Timed           bool        `json:"timed"`
	DefaultDuration uint32      `json:"defaultDuration"` // frames; 20 per second
	Stacking        string      `json:"stacking"`
	Valence         string      `json:"valence"`
	Params          []ParamSpec `json:"params"`
}

// HookInfo describes one hook, as reported by hook.list. IDFilter is whether an
// id filter narrows it; FilterField names the field it matches on
// ("sceneId"/"itemId"/"actorId"), "" when not filterable — one source of truth
// so consumers needn't hardcode it.
type HookInfo struct {
	Name        string `json:"name"`
	IDFilter    bool   `json:"idFilter"`
	FilterField string `json:"filterField,omitempty"`
}

// HookName identifies a subscribable game event. This is the compiled-in
// catalog; hook.list is the live authoritative source (the game's table can
// move ahead of this app). An unrecognized name isn't a wire error — the game
// just replies OutcomeImpossible.
type HookName string

const (
	HookOnSceneInit      HookName = "OnSceneInit"
	HookOnItemGive       HookName = "OnItemGive"
	HookOnActorInit      HookName = "OnActorInit"
	HookOnFlagSet        HookName = "OnFlagSet"
	HookOnFlagUnset      HookName = "OnFlagUnset"
	HookOnSceneFlagSet   HookName = "OnSceneFlagSet"
	HookOnSceneFlagUnset HookName = "OnSceneFlagUnset"
)

// KnownHookNames is the compiled-in hook catalog. OnSceneInit, OnItemGive, and
// OnActorInit accept a per-id filter; the rest are all-or-nothing.
var KnownHookNames = []HookName{
	HookOnSceneInit,
	HookOnItemGive,
	HookOnActorInit,
	HookOnFlagSet,
	HookOnFlagUnset,
	HookOnSceneFlagSet,
	HookOnSceneFlagUnset,
}

// KnownHookCatalog is the compiled-in catalog: KnownHookNames plus id-filter
// support derived from filterFieldFor (so it can't drift). It's what the Hooks
// editor offers before a game connects; a live hook.list replaces it.
func KnownHookCatalog() []HookInfo {
	catalog := make([]HookInfo, len(KnownHookNames))
	for i, name := range KnownHookNames {
		field := filterFieldFor(name)
		catalog[i] = HookInfo{Name: string(name), IDFilter: field != "", FilterField: field}
	}
	return catalog
}

// Hook is an unsolicited game event. Only the fields relevant to Type are set
// by the game (its Sail.cpp handlers); the rest stay zero. What each sends:
//
//	OnSceneInit                      sceneId
//	OnItemGive                       itemId
//	OnActorInit                      actorId, params
//	OnFlagSet / OnFlagUnset          flagType, flag
//	OnSceneFlagSet / OnSceneFlagUnset sceneId, flagType, flag
type Hook struct {
	Type     string `json:"type"`
	SceneID  int    `json:"sceneId"`
	ItemID   int    `json:"itemId"`
	ActorID  int    `json:"actorId"`
	Params   int    `json:"params"`
	FlagType int    `json:"flagType"`
	Flag     int    `json:"flag"`
}

// filterFieldFor returns the field an id filter matches on — "sceneId",
// "itemId", "actorId" — or "" when the hook isn't filterable. Mirrors the
// game's Sail.cpp dispatch, and is the single source for Hook.FilterID (value)
// and HookInfo.FilterField (name).
func filterFieldFor(name HookName) string {
	switch name {
	case HookOnSceneInit:
		return "sceneId"
	case HookOnItemGive:
		return "itemId"
	case HookOnActorInit:
		return "actorId"
	default:
		return ""
	}
}

// FilterID returns the id an id-filter matches for this hook, and whether it's
// filterable (see filterFieldFor). Sail subscribes unfiltered, so a binding's
// id filter is compared here locally when a hook arrives.
func (h Hook) FilterID() (id int, filterable bool) {
	switch filterFieldFor(HookName(h.Type)) {
	case "sceneId":
		return h.SceneID, true
	case "itemId":
		return h.ItemID, true
	case "actorId":
		return h.ActorID, true
	default:
		return 0, false
	}
}

// envelope routes a packet before decoding the rest. It omits "id"
// deliberately: the game stamps hook packets with a numeric std::rand() id,
// not a string, and routing never needs it.
type envelope struct {
	Type PacketType `json:"type"`
}

// commandPacket is the outgoing "run this raw console command" packet.
type commandPacket struct {
	ID      string     `json:"id"`
	Type    PacketType `json:"type"`
	Command string     `json:"command"`
}

// ActionRequest is an action.apply request. Duration and ExpiresAfter are
// frames (20/sec); nil uses the game's defaults (the action's DefaultDuration,
// a 30s readiness window). Empty Lifetime defaults to LifetimeSession.
type ActionRequest struct {
	Name         string
	Params       map[string]any
	Duration     *uint32
	ExpiresAfter *uint32
	Lifetime     Lifetime
}

// actionApplyPacket is the outgoing "action.apply" packet.
type actionApplyPacket struct {
	ID           string         `json:"id"`
	Type         PacketType     `json:"type"`
	Name         string         `json:"name"`
	Params       map[string]any `json:"params,omitempty"`
	Duration     *uint32        `json:"duration,omitempty"`
	ExpiresAfter *uint32        `json:"expiresAfter,omitempty"`
	Lifetime     Lifetime       `json:"lifetime,omitempty"`
}

// actionRemovePacket is the outgoing "action.remove" packet.
type actionRemovePacket struct {
	ID   string     `json:"id"`
	Type PacketType `json:"type"`
	Name string     `json:"name"`
}

// queryPacket is the outgoing shape for the no-argument queries:
// action.list, action.status, and hook.list.
type queryPacket struct {
	ID   string     `json:"id"`
	Type PacketType `json:"type"`
}

// subscriptionPacket is the outgoing subscribe/unsubscribe (Type distinguishes
// them). HookIDFilter, when set, narrows to a single scene/item/actor id (only
// for filterable hooks); omitted means every occurrence of HookName.
//
// Protocol v2 renamed these wire fields: eventName→hookName and
// eventIdFilter→hookIdFilter, matching hook.list and the hook packet.
type subscriptionPacket struct {
	ID           string     `json:"id"`
	Type         PacketType `json:"type"`
	HookName     HookName   `json:"hookName"`
	HookIDFilter *int       `json:"hookIdFilter,omitempty"`
}

// resultPacket is the ack for a packet we sent — a superset of every reply
// shape; each request type fills a different subset, the rest stay zero.
// Results echo our own string id, so ID stays a string here (unlike hook
// packets).
type resultPacket struct {
	ID        string       `json:"id"`
	Type      PacketType   `json:"type"`
	Outcome   Outcome      `json:"outcome"`
	Reason    string       `json:"reason,omitempty"`
	Actions   []ActionInfo `json:"actions,omitempty"`   // action.list
	Ready     bool         `json:"ready,omitempty"`     // action.status
	Pending   int          `json:"pending,omitempty"`   // action.status
	Active    int          `json:"active,omitempty"`    // action.status
	Hooks     []HookInfo   `json:"hooks,omitempty"`     // hook.list
	Cancelled int          `json:"cancelled,omitempty"` // action.remove
}

// actionEndedPacket is the unsolicited follow-up to a timed action.apply:
// after the initial "applied" result, a second message with the same id
// arrives when the action finishes or is cancelled.
type actionEndedPacket struct {
	ID      string     `json:"id"`
	Type    PacketType `json:"type"`
	Outcome Outcome    `json:"outcome"`
}

// hookPacket is an incoming game event. It omits "id" — hook ids are numeric
// and fire-and-forget, never correlated back.
type hookPacket struct {
	Type PacketType `json:"type"`
	Hook Hook       `json:"hook"`
}
