package main

import (
	"testing"

	"github.com/HarbourMasters/Sail/internal/config"
	"github.com/HarbourMasters/Sail/internal/sail"
)

func TestRunScriptEmitsActionWithTypedParams(t *testing.T) {
	steps, err := runScript(`sail.action("gravity", { level: 3, wobble: 1.5, tag: "x", on: true })`,
		triggerContext{User: "garrett"})
	if err != nil {
		t.Fatalf("runScript: %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("want 1 step, got %d", len(steps))
	}

	step := steps[0]
	if step.Kind != config.StepKindAction || step.Name != "gravity" {
		t.Fatalf("unexpected step kind/name: %+v", step)
	}
	// goja exports whole JS numbers as int64 and fractional ones as float64;
	// both marshal to JSON numbers on the wire.
	if got := step.Params["level"]; got != int64(3) {
		t.Errorf("level = %#v, want int64(3)", got)
	}
	if got := step.Params["wobble"]; got != 1.5 {
		t.Errorf("wobble = %#v, want 1.5", got)
	}
	if got := step.Params["tag"]; got != "x" {
		t.Errorf("tag = %#v, want \"x\"", got)
	}
	if got := step.Params["on"]; got != true {
		t.Errorf("on = %#v, want true", got)
	}
}

func TestRunScriptDurationSecondsToFramesAndLifetime(t *testing.T) {
	steps, err := runScript(`sail.action("rave", {}, { duration: 30, lifetime: "save" })`, triggerContext{})
	if err != nil {
		t.Fatalf("runScript: %v", err)
	}
	if steps[0].Duration == nil || *steps[0].Duration != 30*framesPerSecond {
		t.Errorf("duration = %v, want %d frames", steps[0].Duration, 30*framesPerSecond)
	}
	if steps[0].Lifetime != sail.LifetimeSave {
		t.Errorf("lifetime = %q, want %q", steps[0].Lifetime, sail.LifetimeSave)
	}
}

func TestRunScriptCommandAndRemove(t *testing.T) {
	steps, err := runScript(`sail.command("givehealth 20"); sail.remove("gravity")`, triggerContext{})
	if err != nil {
		t.Fatalf("runScript: %v", err)
	}
	if len(steps) != 2 {
		t.Fatalf("want 2 steps, got %d", len(steps))
	}
	if steps[0].Kind != config.StepKindCommand || steps[0].Command != "givehealth 20" {
		t.Errorf("step 0 = %+v", steps[0])
	}
	if steps[1].Kind != config.StepKindRemove || steps[1].Name != "gravity" {
		t.Errorf("step 1 = %+v", steps[1])
	}
}

func TestRunScriptUsesTriggerContext(t *testing.T) {
	// A realistic script: branch on the trigger, use an arg as a param.
	script := `
		if (trigger.args.length > 0) {
			sail.action("gravity", { level: parseInt(trigger.args[0], 10) });
		} else {
			sail.command("hello " + trigger.user);
		}
	`
	steps, err := runScript(script, triggerContext{User: "garrett", Message: "5", Args: []string{"5"}})
	if err != nil {
		t.Fatalf("runScript: %v", err)
	}
	if len(steps) != 1 || steps[0].Params["level"] != int64(5) {
		t.Fatalf("want gravity level 5, got %+v", steps)
	}
}

func TestRunScriptBareTriggerHasEmptyArgs(t *testing.T) {
	// nil Args must reach JS as an empty array, not null, so .length works.
	steps, err := runScript(`sail.command("args=" + trigger.args.length)`, triggerContext{})
	if err != nil {
		t.Fatalf("runScript: %v", err)
	}
	if steps[0].Command != "args=0" {
		t.Errorf("command = %q, want \"args=0\"", steps[0].Command)
	}
}

func TestRunScriptThrowIsReported(t *testing.T) {
	_, err := runScript(`throw new Error("boom")`, triggerContext{})
	if err == nil {
		t.Fatal("want error from a throwing script, got nil")
	}
}

func TestRunScriptRunawayLoopIsInterrupted(t *testing.T) {
	// Must return (via the timeout interrupt) rather than hang the test.
	_, err := runScript(`while (true) {}`, triggerContext{})
	if err == nil {
		t.Fatal("want interrupt error from an infinite loop, got nil")
	}
}

func TestFireBindingScriptErrorPropagates(t *testing.T) {
	// A failing script must surface a non-nil error (for the activity feed).
	// The error path returns before fan-out, so a bare App is enough.
	a := &App{}
	err := a.fireBinding(config.Binding{Script: `throw new Error("nope")`}, triggerContext{})
	if err == nil {
		t.Fatal("want error from a failing script binding, got nil")
	}
}

func TestFireBindingSucceedsWithNoClients(t *testing.T) {
	a := &App{sailServer: sail.NewServer()}

	// Static steps: fanOut sees no clients and returns; no error.
	if err := a.fireBinding(config.Binding{Steps: []config.Step{{Kind: config.StepKindCommand, Command: "x"}}}, triggerContext{}); err != nil {
		t.Fatalf("static-step binding errored: %v", err)
	}
	// A valid script that emits a step: runs, fans out to nobody, no error.
	if err := a.fireBinding(config.Binding{Script: `sail.command("x")`}, triggerContext{}); err != nil {
		t.Fatalf("valid script binding errored: %v", err)
	}
}

func TestValidateScript(t *testing.T) {
	if err := validateScript(""); err != nil {
		t.Errorf("empty script should be valid: %v", err)
	}
	if err := validateScript(`sail.command("ok")`); err != nil {
		t.Errorf("valid script rejected: %v", err)
	}
	if err := validateScript(`sail.command(`); err == nil {
		t.Error("want syntax error for unbalanced parens, got nil")
	}
}
