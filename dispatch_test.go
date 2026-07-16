package main

import (
	"testing"

	"github.com/HarbourMasters/Sail/internal/config"
	"github.com/HarbourMasters/Sail/internal/sail"
)

func intPtr(n int) *int { return &n }

// TestHookBindingMatches covers the hook dispatch rule: name match, id filter,
// and that a filter on a non-id-filterable hook never matches.
func TestHookBindingMatches(t *testing.T) {
	itemGive10 := sail.Hook{Type: "OnItemGive", ItemID: 10}
	flagSet := sail.Hook{Type: "OnFlagSet", FlagType: 1, Flag: 5}

	cases := []struct {
		name string
		hb   config.HookBinding
		hook sail.Hook
		want bool
	}{
		{"name mismatch", config.HookBinding{HookName: "OnSceneInit"}, itemGive10, false},
		{"name match, no filter fires on any", config.HookBinding{HookName: "OnItemGive"}, itemGive10, true},
		{"name match is case-insensitive", config.HookBinding{HookName: "onitemgive"}, itemGive10, true},
		{"filter matches the id", config.HookBinding{HookName: "OnItemGive", IDFilter: intPtr(10)}, itemGive10, true},
		{"filter with the wrong id", config.HookBinding{HookName: "OnItemGive", IDFilter: intPtr(11)}, itemGive10, false},
		{"filter on a non-filterable hook never matches", config.HookBinding{HookName: "OnFlagSet", IDFilter: intPtr(5)}, flagSet, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := hookBindingMatches(tc.hb, tc.hook); got != tc.want {
				t.Errorf("hookBindingMatches = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestExpandHookVariables checks hook fields reach templates: as text, coerced
// to a number for a param, and an unknown name left verbatim.
func TestExpandHookVariables(t *testing.T) {
	hook := sail.Hook{Type: "OnActorInit", ActorID: 80, Params: 3}
	ctx := triggerContext{Hook: &hook}

	if got := ctx.expand("actor {{actorId}} params {{params}}"); got != "actor 80 params 3" {
		t.Errorf("expand = %q, want %q", got, "actor 80 params 3")
	}
	if got := resolveParam("{{actorId}}", ctx); got != 80 {
		t.Errorf("resolveParam({{actorId}}) = %#v, want int 80", got)
	}
	if got := ctx.expand("{{bogus}}"); got != "{{bogus}}" {
		t.Errorf("expand(unknown var) = %q, want it left verbatim", got)
	}
}

// TestRunScriptHookTrigger confirms a script sees the hook's fields on
// `trigger` (and not the chat-only user/message/args).
func TestRunScriptHookTrigger(t *testing.T) {
	hook := sail.Hook{Type: "OnItemGive", ItemID: 4}
	steps, err := runScript(
		`if (trigger.hook === "OnItemGive" && trigger.itemId === 4) { sail.action("gravity"); }`,
		triggerContext{Hook: &hook},
	)
	if err != nil {
		t.Fatalf("runScript: %v", err)
	}
	if len(steps) != 1 || steps[0].Name != "gravity" {
		t.Fatalf("want one gravity step, got %+v", steps)
	}
}
