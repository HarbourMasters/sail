package main

import (
	"strings"
	"sync"
	"time"

	"github.com/HarbourMasters/Sail/internal/config"
	"github.com/HarbourMasters/Sail/internal/sail"
	"github.com/dop251/goja"
)

const (
	// scriptTimeout bounds a script's run before it's interrupted. A script only
	// turns a trigger into a few steps, so this is deliberately short — it
	// exists to stop a runaway loop from wedging the calling goroutine.
	scriptTimeout = 200 * time.Millisecond

	// framesPerSecond converts the seconds a script author writes durations in
	// to the frames the wire protocol carries (mirrors the game's 20/sec).
	framesPerSecond = 20
)

// scriptAPI is the host side of the `sail` object a script calls. Each method
// appends one config.Step, read back after the script returns. The methods run
// only on the script's single goroutine, so no locking.
type scriptAPI struct {
	steps []config.Step
}

// command queues a raw console command: sail.command("givehealth 20").
func (s *scriptAPI) command(call goja.FunctionCall) goja.Value {
	s.steps = append(s.steps, config.Step{
		Kind:    config.StepKindCommand,
		Command: call.Argument(0).String(),
	})
	return goja.Undefined()
}

// remove queues cancelling a running action: sail.remove("gravity").
func (s *scriptAPI) remove(call goja.FunctionCall) goja.Value {
	s.steps = append(s.steps, config.Step{
		Kind: config.StepKindRemove,
		Name: call.Argument(0).String(),
	})
	return goja.Undefined()
}

// chat queues a Twitch chat message posted as the logged-in user. Like the
// others it only declares intent — the send happens host-side after the script
// returns, so the sandbox never touches the network.
func (s *scriptAPI) chat(call goja.FunctionCall) goja.Value {
	s.steps = append(s.steps, config.Step{
		Kind:    config.StepKindChat,
		Message: call.Argument(0).String(),
	})
	return goja.Undefined()
}

// action queues starting (or re-triggering) an action:
//
//	sail.action("gravity")
//	sail.action("gravity", { level: 3 })
//	sail.action("gravity", { level: 3 }, { duration: 30, lifetime: "save" })
//
// The params object maps straight onto the action's param schema; JS numbers,
// strings and booleans arrive as the int/float/string/bool the game expects,
// so unlike the {{...}} template path there's nothing to coerce. The optional
// third argument overrides the action's own defaults (see applyActionOptions).
func (s *scriptAPI) action(call goja.FunctionCall) goja.Value {
	step := config.Step{Kind: config.StepKindAction, Name: call.Argument(0).String()}

	if params, ok := exportObject(call.Argument(1)); ok {
		step.Params = params
	}
	if opts, ok := exportObject(call.Argument(2)); ok {
		applyActionOptions(&step, opts)
	}

	s.steps = append(s.steps, step)
	return goja.Undefined()
}

// exportObject returns v as a Go map when it's a present JS object, so a
// missing/undefined/null/non-object argument is treated as "not given".
func exportObject(v goja.Value) (map[string]any, bool) {
	if v == nil || goja.IsUndefined(v) || goja.IsNull(v) {
		return nil, false
	}
	m, ok := v.Export().(map[string]any)
	return m, ok
}

// applyActionOptions reads sail.action's optional { duration?: seconds,
// lifetime? } onto the step. Duration is authored in seconds and converted to
// frames; omitted leaves the game's default.
func applyActionOptions(step *config.Step, opts map[string]any) {
	if seconds, ok := toFloat(opts["duration"]); ok {
		frames := uint32(seconds * framesPerSecond)
		step.Duration = &frames
	}
	if lifetime, ok := opts["lifetime"].(string); ok && lifetime == string(sail.LifetimeSave) {
		step.Lifetime = sail.LifetimeSave
	}
}

// toFloat pulls a float from a goja-exported JS number: goja exports whole
// numbers as int64 and fractional as float64, so both are handled.
func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case int64:
		return float64(n), true
	case float64:
		return n, true
	default:
		return 0, false
	}
}

// runScript runs a binding's JavaScript against the trigger and returns the
// steps it emitted. The script gets a read-only `trigger`. For a chat/redeem —
//
//	trigger.user     display name of whoever triggered it
//	trigger.message  the message text (redeem input, or chat after the trigger word)
//	trigger.args     trigger.message split into words
//
// — or for a game hook, the hook's fields (see sail.Hook) —
//
//	trigger.hook     the hook name, e.g. "OnItemGive"
//	trigger.itemId / .sceneId / .actorId / .params / .flagType / .flag
//
// — then calls sail.action / .command / .remove / .chat to build the steps.
//
// The runtime is a throwaway with no host access (no require, fs, network,
// timers, console). goja isn't goroutine-safe, so this runs to completion
// before the steps fan out; a timer goroutine interrupts it past scriptTimeout.
func runScript(script string, ctx triggerContext) ([]config.Step, error) {
	program, err := compileScript(script)
	if err != nil {
		return nil, err
	}

	vm := goja.New()

	api := &scriptAPI{}
	sailObj := vm.NewObject()
	_ = sailObj.Set("action", api.action)
	_ = sailObj.Set("command", api.command)
	_ = sailObj.Set("remove", api.remove)
	_ = sailObj.Set("chat", api.chat)
	_ = vm.Set("sail", sailObj)

	// A nil slice surfaces in JS as null, so trigger.args.length/.map would
	// throw for a bare trigger.
	args := ctx.Args
	if args == nil {
		args = []string{}
	}
	trigger := map[string]any{
		"user":    ctx.User,
		"message": ctx.Message,
		"args":    args,
	}
	// A hook trigger exposes the hook's fields instead of user/message/args.
	if ctx.Hook != nil {
		trigger = map[string]any{
			"hook":     ctx.Hook.Type,
			"sceneId":  ctx.Hook.SceneID,
			"itemId":   ctx.Hook.ItemID,
			"actorId":  ctx.Hook.ActorID,
			"params":   ctx.Hook.Params,
			"flagType": ctx.Hook.FlagType,
			"flag":     ctx.Hook.Flag,
		}
	}
	_ = vm.Set("trigger", trigger)

	// The timer interrupts a runaway script; if it finishes first, the deferred
	// Stop cancels the timer.
	timer := time.AfterFunc(scriptTimeout, func() {
		vm.Interrupt("script exceeded its time limit")
	})
	defer timer.Stop()

	if _, err := vm.RunProgram(program); err != nil {
		return nil, err
	}
	return api.steps, nil
}

// validateScript compiles a script without running it, so a syntax error
// surfaces on save rather than on the next fire. Runtime errors can't be caught
// here (they hit the activity feed when fired). An empty script is valid — the
// binding uses its static Steps.
func validateScript(script string) error {
	if strings.TrimSpace(script) == "" {
		return nil
	}
	// compileScript both validates and warms the cache, so a binding saved
	// here is already compiled by the time it first fires.
	_, err := compileScript(script)
	return err
}

// compiledScripts caches the *goja.Program per source, so a script is compiled
// once, not on every fire — the compile dominates the cost, and a high-frequency
// hook would otherwise recompile the same source hundreds of times per scene
// load. A Program is immutable and safe to share; the per-fire goja.New() stays
// (a runtime can't be cheaply reset).
var compiledScripts sync.Map // script source -> *goja.Program

func compileScript(script string) (*goja.Program, error) {
	if cached, ok := compiledScripts.Load(script); ok {
		return cached.(*goja.Program), nil
	}
	program, err := goja.Compile("binding", script, false)
	if err != nil {
		return nil, err
	}
	compiledScripts.Store(script, program)
	return program, nil
}
