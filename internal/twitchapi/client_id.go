package twitchapi

import "os"

// compiledInClientID is the app's Twitch client ID, baked into the binary (a
// public OAuth client has no secret to protect). To use your own, register a
// Public app at https://dev.twitch.tv/console/apps with OAuth Redirect URL
// http://localhost:43385/callback, then set this or the SAIL_TWITCH_CLIENT_ID
// env var (no rebuild).
const compiledInClientID = "n1l2wkc6dz3oroigsvheo4iv3cmee1"

// RedirectURI is the loopback address Twitch redirects back to after login.
// It must exactly match what's registered on the Twitch application.
const RedirectURI = "http://localhost:43385/callback"

// Scopes requested: read the broadcaster's chat and channel point redemptions,
// and post chat as them.
var Scopes = []string{"user:read:chat", "user:write:chat", "channel:read:redemptions"}

// ClientID returns the configured Twitch application client ID, preferring
// SAIL_TWITCH_CLIENT_ID over the compiled-in value.
func ClientID() string {
	if id := os.Getenv("SAIL_TWITCH_CLIENT_ID"); id != "" {
		return id
	}
	return compiledInClientID
}

// Configured reports whether a client ID is set (non-empty).
func Configured() bool {
	return ClientID() != ""
}
