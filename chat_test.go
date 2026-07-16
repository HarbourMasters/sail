package main

import (
	"testing"

	"github.com/HarbourMasters/Sail/internal/config"
	"github.com/HarbourMasters/Sail/internal/twitchapi"
)

// TestResolveChatStepExpandsMessage checks a static "post chat message" step
// gets its {{...}} variables filled from the trigger, the way a command does.
func TestResolveChatStepExpandsMessage(t *testing.T) {
	step := config.Step{Kind: config.StepKindChat, Message: "Thanks {{user}}!"}
	got := resolveStep(step, triggerContext{User: "garrett"})
	if got.Message != "Thanks garrett!" {
		t.Errorf("message = %q, want %q", got.Message, "Thanks garrett!")
	}
}

// TestRunScriptChat checks sail.chat queues a chat step built from the trigger.
func TestRunScriptChat(t *testing.T) {
	steps, err := runScript(`sail.chat("hi " + trigger.user)`, triggerContext{User: "garrett"})
	if err != nil {
		t.Fatalf("runScript: %v", err)
	}
	if len(steps) != 1 || steps[0].Kind != config.StepKindChat || steps[0].Message != "hi garrett" {
		t.Fatalf("want one chat step \"hi garrett\", got %+v", steps)
	}
}

// TestChatEchoGuard covers the loop guard: Sail's own echo is skipped, but a
// viewer's identical message (and anything we didn't post) still fires.
func TestChatEchoGuard(t *testing.T) {
	a := NewApp()
	a.session = &twitchapi.StoredSession{UserID: "42"}
	a.rememberSentChat("!kick")

	echo := func(userID, text string) twitchapi.ChatMessageEvent {
		e := twitchapi.ChatMessageEvent{ChatterUserID: userID}
		e.Message.Text = text
		return e
	}

	if !a.isOwnEcho(echo("42", "!kick")) {
		t.Error("our own posted message echoing back should be recognized")
	}
	if a.isOwnEcho(echo("99", "!kick")) {
		t.Error("a viewer's identical message should not be treated as our echo")
	}
	if a.isOwnEcho(echo("42", "hello")) {
		t.Error("a message we never posted should not be treated as our echo")
	}
}
