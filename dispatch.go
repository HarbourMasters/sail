package main

import (
	"context"
	"log"
	"regexp"
	"strconv"
	"strings"

	"github.com/HarbourMasters/Sail/internal/config"
	"github.com/HarbourMasters/Sail/internal/sail"
)

// placeholder matches a {{name}} reference — digits ({{0}}…, a positional
// word) or a named variable ({{message}}, {{user}}).
var placeholder = regexp.MustCompile(`\{\{(\w+)\}\}`)

// triggerContext is what a binding's templates can reference about what fired
// it. Only one source is set: a chat/redeem trigger fills User/Message/Args and
// leaves Hook nil; a hook trigger sets Hook and leaves the rest empty.
type triggerContext struct {
	User    string     // display name of whoever triggered it -> {{user}}
	Message string     // the whole message text they sent      -> {{message}}
	Args    []string   // Message split into words              -> {{0}}, {{1}}, …
	Hook    *sail.Hook // the game hook that fired it, if any    -> {{itemId}}, {{sceneId}}, …
}

// expand substitutes {{...}} references: {{n}} is the n-th message word (or "0"
// out of range, so a numeric param still gets a number when the arg is
// omitted); {{message}}/{{user}} the named chat vars; hook fields for a hook
// trigger (see hookVariable). An unknown {{name}} is left verbatim so typos
// stay visible.
func (ctx triggerContext) expand(text string) string {
	return placeholder.ReplaceAllStringFunc(text, func(match string) string {
		// match is always "{{name}}", so slice off the delimiters rather than
		// running the regexp again.
		name := match[2 : len(match)-2]

		if index, err := strconv.Atoi(name); err == nil {
			if index >= 0 && index < len(ctx.Args) {
				return ctx.Args[index]
			}
			return "0"
		}

		switch name {
		case "message":
			return ctx.Message
		case "user":
			return ctx.User
		}

		if ctx.Hook != nil {
			if value, ok := hookVariable(*ctx.Hook, name); ok {
				return value
			}
		}
		return match
	})
}

// hookVariable resolves a hook trigger's {{...}} variables. Every field name
// resolves regardless of the hook (an unpopulated one is just "0"); the editor
// only advertises the subset each hook actually populates.
func hookVariable(h sail.Hook, name string) (string, bool) {
	switch name {
	case "hook":
		return h.Type, true
	case "sceneId":
		return strconv.Itoa(h.SceneID), true
	case "itemId":
		return strconv.Itoa(h.ItemID), true
	case "actorId":
		return strconv.Itoa(h.ActorID), true
	case "params":
		return strconv.Itoa(h.Params), true
	case "flagType":
		return strconv.Itoa(h.FlagType), true
	case "flag":
		return strconv.Itoa(h.Flag), true
	default:
		return "", false
	}
}

// resolveStep expands {{...}} placeholders in a step's command or templated
// param values against the trigger that fired the binding.
func resolveStep(step config.Step, ctx triggerContext) config.Step {
	switch step.Kind {
	case config.StepKindCommand:
		step.Command = ctx.expand(step.Command)

	case config.StepKindChat:
		step.Message = ctx.expand(step.Message)

	case config.StepKindAction:
		if len(step.Params) == 0 {
			break
		}
		resolved := make(map[string]any, len(step.Params))
		for key, value := range step.Params {
			resolved[key] = resolveParam(value, ctx)
		}
		step.Params = resolved
	}
	return step
}

// resolveParam expands {{...}} in a templated string param, converting the
// result to a number when it parses as one so an int/float param still arrives
// numeric. Non-string values, and strings without a {{...}}, pass through.
func resolveParam(value any, ctx triggerContext) any {
	text, isString := value.(string)
	if !isString || !placeholder.MatchString(text) {
		return value
	}

	expanded := ctx.expand(text)
	if number, err := strconv.Atoi(expanded); err == nil {
		return number
	}
	if number, err := strconv.ParseFloat(expanded, 64); err == nil {
		return number
	}
	switch expanded {
	case "true":
		return true
	case "false":
		return false
	}
	return expanded
}

// fireBinding produces a binding's steps — from its Script (run against the
// trigger) or its static Steps with {{...}} resolved — and dispatches them. The
// error is non-nil only for a failed script; the static path never errors.
func (a *App) fireBinding(binding config.Binding, ctx triggerContext) error {
	var steps []config.Step
	if strings.TrimSpace(binding.Script) != "" {
		// Script output is already final — no template pass (which would
		// mangle a script that legitimately emitted a literal "{{").
		produced, err := runScript(binding.Script, ctx)
		if err != nil {
			return err
		}
		steps = produced
	} else {
		steps = make([]config.Step, len(binding.Steps))
		for i, step := range binding.Steps {
			steps[i] = resolveStep(step, ctx)
		}
	}

	a.dispatchSteps(steps)
	return nil
}

// fireTrigger fires a binding and records it on the activity feed, capturing a
// script error onto the event so a broken script surfaces on the Dashboard.
// Every trigger source routes through here so the error-to-activity wiring
// can't be forgotten; cooldown gating stays at the call site, where the key
// (and, for hooks, gating before the goroutine) differs.
func (a *App) fireTrigger(binding config.Binding, ctx triggerContext, activity ActivityEvent) {
	if err := a.fireBinding(binding, ctx); err != nil {
		activity.Error = err.Error()
	}
	a.emitActivity(activity)
}

// dispatchSteps routes finished steps: a chat step posts to Twitch (once,
// regardless of any game connection), everything else fans out to the clients.
func (a *App) dispatchSteps(steps []config.Step) {
	gameSteps := make([]config.Step, 0, len(steps))
	for _, step := range steps {
		if step.Kind == config.StepKindChat {
			if strings.TrimSpace(step.Message) != "" {
				go a.postChatMessage(step.Message)
			}
			continue
		}
		gameSteps = append(gameSteps, step)
	}
	a.fanOut(gameSteps)
}

// fanOut sends steps to every client, one goroutine per client so a slow one
// can't stall the caller or the others. Within a client, steps run in order,
// one at a time (per-step goroutines would leave order to the scheduler).
func (a *App) fanOut(steps []config.Step) {
	clients := a.sailServer.Clients()
	if len(clients) == 0 {
		return
	}

	for _, client := range clients {
		go func(c *sail.Client) {
			for _, step := range steps {
				sendStep(c, step)
			}
		}(client)
	}
}

func sendStep(c *sail.Client, step config.Step) {
	ctx := context.Background()

	switch step.Kind {
	case config.StepKindCommand:
		outcome, err := c.SendCommand(ctx, step.Command)
		logStepResult(c, "command", outcome, err, sail.OutcomeOK)

	case config.StepKindAction:
		req := sail.ActionRequest{Name: step.Name, Params: step.Params, Duration: step.Duration, Lifetime: step.Lifetime}
		_, outcome, err := c.ApplyAction(ctx, req)
		logStepResult(c, "action.apply "+step.Name, outcome, err, sail.OutcomeApplied)

	case config.StepKindRemove:
		_, outcome, err := c.RemoveAction(ctx, step.Name)
		logStepResult(c, "action.remove "+step.Name, outcome, err, sail.OutcomeOK)
	}
}

func logStepResult(c *sail.Client, what string, outcome sail.Outcome, err error, wantOutcome sail.Outcome) {
	if err != nil {
		log.Printf("send %s to client %d failed: %v", what, c.ID, err)
	} else if outcome != wantOutcome {
		log.Printf("client %d responded %q to %s", c.ID, outcome, what)
	}
}

// handleHook fires every binding matching an arriving hook. It runs on the
// client's read goroutine, so it only scans inline and hands firing off to a
// goroutine — a script can run up to scriptTimeout, and a high-frequency hook
// must not stall the read loop.
func (a *App) handleHook(hook sail.Hook) {
	bindings := a.findHookBindings(hook)

	// Gate cooldowns here on the read loop (serialized) rather than in the
	// goroutine — else a cooldowned high-frequency hook spawns a goroutine per
	// fire just to fail the check. Filter in place: the slice is already fresh.
	ready := bindings[:0]
	for _, hb := range bindings {
		if a.checkCooldown("hook:"+hb.ID, hb.Binding.CooldownSeconds) {
			ready = append(ready, hb)
		}
	}
	if len(ready) == 0 {
		return
	}

	go a.fireHookBindings(hook, ready)
}

// fireHookBindings fires each already-cooldown-gated binding against the hook,
// reporting to the activity feed like a chat or redeem firing.
func (a *App) fireHookBindings(hook sail.Hook, bindings []config.HookBinding) {
	for _, hb := range bindings {
		h := hook
		a.fireTrigger(hb.Binding, triggerContext{Hook: &h}, ActivityEvent{Source: "hook", Trigger: describeHookBinding(hb)})
	}
}

// findHookBindings returns every configured binding matching hook — more than
// one can (an unfiltered and an id-filtered binding), and all of them fire.
func (a *App) findHookBindings(hook sail.Hook) []config.HookBinding {
	a.configMu.Lock()
	defer a.configMu.Unlock()

	var matches []config.HookBinding
	for _, hb := range a.config.Hooks {
		if hookBindingMatches(hb, hook) {
			matches = append(matches, hb)
		}
	}
	return matches
}

// hookBindingMatches reports whether hb fires for hook: names must match, and
// an id filter (when set) must equal the hook's dispatch id. A filter on a
// non-id-filterable hook never matches.
func hookBindingMatches(hb config.HookBinding, hook sail.Hook) bool {
	if !strings.EqualFold(hb.HookName, hook.Type) {
		return false
	}
	if hb.IDFilter == nil {
		return true
	}
	id, filterable := hook.FilterID()
	return filterable && id == *hb.IDFilter
}

// describeHookBinding is the activity-feed label for a hook firing: the hook
// name, plus its id filter when it has one.
func describeHookBinding(hb config.HookBinding) string {
	if hb.IDFilter != nil {
		return hb.HookName + " · id " + strconv.Itoa(*hb.IDFilter)
	}
	return hb.HookName
}
