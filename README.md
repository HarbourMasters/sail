# Sail

> [!NOTE]
> **Disclaimer:** This is a mostly an AI-generated project, written largely by Claude Opus 4.8. The code may contain
> bugs, security vulnerabilities, or other issues. This project was deemed low-stakes and suitable for this. Please use AI responsibly - ProxySaw

Sail is a crowd-control desktop app for [2 Ship 2 Harkinian](https://github.com/HarbourMasters/2ship2harkinian):
viewers trigger in-game effects with Twitch chat commands and channel point redemptions. It's a
[Wails](https://wails.io) app (Go backend, React/TypeScript frontend) that authenticates with Twitch, lets you
map chat commands (`!kick 2`) and redeems to game actions through a UI, and hosts the TCP server the game
connects to.

The **Sail protocol** the game speaks is general-purpose: it lets any external program trigger in-game actions
and react to game events. Twitch viewer interaction is the obvious use case — and what this app implements —
but the same protocol could drive the game from anything. That protocol (below) is defined in 2ship2harkinian's
own `mm/2s2h/Network/Sail/Sail.cpp`, which is under active development — check that file before assuming a shape
here is still current.

## Downloads
- [Linux](https://nightly.link/HarbourMasters/sail/workflows/ci/main/Sail-linux-amd64.tar.gz.zip)
- [MacOS](https://nightly.link/HarbourMasters/sail/workflows/ci/main/Sail-macos-universal.zip)
- [Windows](https://nightly.link/HarbourMasters/sail/workflows/ci/main/Sail-windows-amd64.zip)

## Development setup

- Install [Go](https://go.dev) 1.25+ and [Node.js](https://nodejs.org).
- Install the Wails CLI: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`.
- `wails dev` for live development, or `wails build` for a release binary in `build/bin`.

### Using your own Twitch application (optional)

The built-in application is shared by everyone running Sail. To point at your own instead — for isolated rate
limits, or if you're forking Sail — register one and override the default:

1. Register an application at the [Twitch Developer Console](https://dev.twitch.tv/console/apps).
2. Set **OAuth Redirect URL** to exactly `http://localhost:43385/callback`.
3. Set **Client Type** to **Public**.
4. Take the generated Client ID and either set it as the `SAIL_TWITCH_CLIENT_ID` environment variable (no
   rebuild needed), or replace the value in `internal/twitchapi/client_id.go` and rebuild.

## Commands, redeems, and hooks

Use the **Commands**, **Redeems**, and **Hooks** pages to map triggers to steps:

- A **command** fires on a chat message starting with its trigger (e.g. `!kick`); a **redeem** fires on its
  channel point reward.
- Any action param or raw command can reference **variables** from the trigger, resolved when the binding fires:
  - `{{message}}` — everything the viewer typed (chat after the trigger word, or a redeem's text input).
  - `{{user}}` — who triggered it.
  - `{{0}}`, `{{1}}`, … — the words of that message. An out-of-range `{{n}}` resolves to `0`, so a numeric param
    still gets a number when chat omits the argument.

  A number/bool param whose editor can't hold `{{…}}` has a `{ }` toggle that swaps in a text field. A templated
  value that resolves to a number is sent as a number, otherwise as text.
- Each binding runs one or more **steps**, immediately and in order: **start an action** (with its params, and
  for timed actions a duration and session/save lifetime), **cancel an action**, **run a raw console command**,
  or **post a chat message**. A timed action's own duration ends it; use a "cancel action" step to end one early.
- The action picker is populated live from the connected game (`action.list`), so it's empty until a game has
  connected at least once.
- An optional **cooldown** rate-limits how often a binding can fire.

### Game hooks

The **Hooks** page fires bindings on in-game events (item pickup, scene load, flag change) rather than Twitch,
over the same protocol (`hook.list`/`subscribe` below). Pick a hook, optionally filter by id, and give it steps
or a script; its fields (`{{itemId}}`, `{{sceneId}}`, …) are available as variables and on `trigger`. A live
feed streams a hook's firings so you can read off ids. Sail subscribes only to hooks you've bound or are
watching.

### Scripting (advanced)

For logic the steps and `{{…}}` variables can't express, a binding's **Script** section takes JavaScript that
produces the steps itself. When it holds any code it runs **instead of** the step list, once per fire. It gets:

- `trigger.user` — who triggered it.
- `trigger.message` — the text they typed.
- `trigger.args` — `trigger.message` split into words.

(A hook binding's `trigger` carries the hook's fields instead — `trigger.itemId`, `trigger.sceneId`, ….)

and calls any of:

- `sail.action(name, params?, options?)` — start an action. `params` matches the action's schema (`{ level: 3 }`);
  `options` is `{ duration, lifetime }`, `duration` in **seconds**, `lifetime` `"session"` (default) or `"save"`.
- `sail.command(string)` — run a raw console command.
- `sail.remove(name)` — cancel a running action.
- `sail.chat(string)` — post a message to your chat.

```js
// !gravity heavy  →  strong, timed gravity; anything else  →  a gentle nudge
if (trigger.args[0] === "heavy") {
  sail.action("gravity", { level: 3 }, { duration: 30 });
} else {
  sail.action("gravity", { level: 1 });
}
```

The runtime is sandboxed (no file, network, or timer access) and a script that runs too long is interrupted.
Syntax errors are caught on save; runtime errors show up in the Dashboard's activity feed when the binding fires.

Configuration is stored as JSON in the OS user config directory (e.g. `~/Library/Application Support/sail` on
macOS), separate from the Twitch session in the keychain.

## The Sail protocol

The game connects over TCP (port 43384 by default) and exchanges NUL-delimited JSON packets. Every outgoing
packet carries an `id`; the game's `result` reply echoes it back.

- **Command** `{"id", "type": "command", "command": "..."}` — run a raw console command. Prefer the action form
  of anything that has one.
- **Start an action** `{"id", "type": "action.apply", "name", "params": {...}, "duration"?, "expiresAfter"?,
  "lifetime"?}` — `params` is keyed by the action's own param names (see `action.list`). `duration`/`expiresAfter`
  are frame counts (20/sec); omitting `duration` uses the action's default, omitting `expiresAfter` gives the
  game 30s to become ready. `lifetime` is `"session"` (default) or `"save"` (dropped when a save loads). A timed
  action reports `applied`, then later an unsolicited `{"id", "type": "action.ended", "outcome"}` with the same
  `id` when it finishes or is cancelled.
- **Cancel an action** `{"id", "type": "action.remove", "name"}` — stops every running instance; `cancelled`
  reports how many.
- **List actions** `{"id", "type": "action.list"}` — the game's live, self-describing action catalog (name,
  display name, timed, default duration, stacking, valence, and a param schema per action). The only source of
  truth, and it needs an active connection.
- **Check readiness** `{"id", "type": "action.status"}` — replies `ready`/`pending`/`active`.
- **List hooks** `{"id", "type": "hook.list"}` — the game's live hook catalog (name + whether it supports a
  per-id filter).
- **Subscribe / unsubscribe** `{"id", "type": "subscribe"|"unsubscribe", "hookName", "hookIdFilter"?}` — hooks are
  opt-in and forgotten on disconnect, so Sail re-subscribes on each connection. `hookIdFilter` narrows delivery
  to one scene/item/actor id.
- **Hook** `{"id", "type": "hook", "hook": {...}}` — an unsolicited game event (scene transition, item pickup,
  flag set, …), sent once subscribed.
- **Result** `{"id", "type": "result", "outcome", "reason"?, ...}` — the game's answer. `outcome` is one word:
  `ok`, `applied`, `finished`/`cancelled`, `expired`, `impossible`, or `invalid` (message was malformed — see
  `reason`).

`internal/sail` implements this protocol standalone, with no dependency on the rest of the app — a reasonable
starting point for another Sail-speaking integration.
