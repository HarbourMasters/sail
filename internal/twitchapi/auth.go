package twitchapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const loginTimeout = 5 * time.Minute

// LoginResult is a successful Twitch login.
type LoginResult struct {
	AccessToken string
	Login       string
	UserID      string
	Scopes      []string
}

// Authenticator runs Twitch's implicit-grant OAuth via a loopback HTTP server:
// opens the browser to Twitch's authorize page, catches the localhost redirect,
// validates the token. Desktop apps are public OAuth clients (no secret), so
// implicit grant — not the authorization code grant — is the flow Twitch
// expects.
type Authenticator struct {
	ClientID    string
	RedirectURI string
	Scopes      []string
}

// Login blocks until the user finishes logging in in their browser, ctx is
// cancelled, or five minutes pass with no response.
func (a *Authenticator) Login(ctx context.Context, openURL func(string) error) (*LoginResult, error) {
	state, err := randomState()
	if err != nil {
		return nil, fmt.Errorf("generate oauth state: %w", err)
	}

	redirectURL, err := url.Parse(a.RedirectURI)
	if err != nil {
		return nil, fmt.Errorf("parse redirect uri: %w", err)
	}

	listener, err := net.Listen("tcp", redirectURL.Host)
	if err != nil {
		return nil, fmt.Errorf("listen on %s (is another Sail instance running?): %w", redirectURL.Host, err)
	}

	type callbackResult struct {
		token string
		err   error
	}
	resultCh := make(chan callbackResult, 1)

	mux := http.NewServeMux()
	mux.HandleFunc(redirectURL.Path, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(callbackPageHTML))
	})
	mux.HandleFunc(redirectURL.Path+"/token", func(w http.ResponseWriter, r *http.Request) {
		defer w.WriteHeader(http.StatusOK)

		q := r.URL.Query()
		if desc := q.Get("error_description"); desc != "" {
			resultCh <- callbackResult{err: fmt.Errorf("twitch login failed: %s", desc)}
			return
		}
		if q.Get("state") != state {
			resultCh <- callbackResult{err: errors.New("login state did not match — please try again")}
			return
		}
		token := q.Get("access_token")
		if token == "" {
			resultCh <- callbackResult{err: errors.New("twitch did not return an access token")}
			return
		}
		resultCh <- callbackResult{token: token}
	})

	server := &http.Server{Handler: mux}
	serveErrCh := make(chan error, 1)
	go func() { serveErrCh <- server.Serve(listener) }()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	if err := openURL(a.authorizeURL(state)); err != nil {
		return nil, fmt.Errorf("open browser for twitch login: %w", err)
	}

	select {
	case res := <-resultCh:
		if res.err != nil {
			return nil, res.err
		}
		return a.finishLogin(ctx, res.token)
	case err := <-serveErrCh:
		return nil, fmt.Errorf("callback server stopped: %w", err)
	case <-time.After(loginTimeout):
		return nil, errors.New("timed out waiting for twitch login")
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (a *Authenticator) authorizeURL(state string) string {
	v := url.Values{}
	v.Set("response_type", "token")
	v.Set("client_id", a.ClientID)
	v.Set("redirect_uri", a.RedirectURI)
	v.Set("scope", strings.Join(a.Scopes, " "))
	v.Set("state", state)
	return "https://id.twitch.tv/oauth2/authorize?" + v.Encode()
}

func (a *Authenticator) finishLogin(ctx context.Context, token string) (*LoginResult, error) {
	validation, err := ValidateToken(ctx, token)
	if err != nil {
		return nil, err
	}
	return &LoginResult{
		AccessToken: token,
		Login:       validation.Login,
		UserID:      validation.UserID,
		Scopes:      validation.Scopes,
	}, nil
}

// TokenValidation is Twitch's answer to "is this token still good".
type TokenValidation struct {
	ClientID string   `json:"client_id"`
	Login    string   `json:"login"`
	Scopes   []string `json:"scopes"`
	UserID   string   `json:"user_id"`
}

// validateTimeout bounds ValidateToken's HTTP call so a dead network fails
// fast — restoreTwitchSession runs it at startup, which mustn't hang.
const validateTimeout = 10 * time.Second

// ValidateToken checks a token against Twitch and reports who it belongs to.
// Implicit-grant tokens carry no refresh token, so this is also how an expired
// stored token is detected.
func ValidateToken(ctx context.Context, token string) (*TokenValidation, error) {
	ctx, cancel := context.WithTimeout(ctx, validateTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://id.twitch.tv/oauth2/validate", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "OAuth "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("validate token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token is not valid (status %d)", resp.StatusCode)
	}

	var v TokenValidation
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		return nil, fmt.Errorf("decode token validation: %w", err)
	}
	return &v, nil
}

func randomState() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

const callbackPageHTML = `<!doctype html>
<html>
<head>
<meta charset="utf-8">
<title>Sail — Twitch Login</title>
<style>
  body { font: 16px system-ui, sans-serif; background:#18181b; color:#efeff1; display:flex; align-items:center; justify-content:center; height:100vh; margin:0; }
</style>
</head>
<body>
<main id="message">Finishing login&hellip;</main>
<script>
  var hash = window.location.hash.replace(/^#/, "");
  var message = document.getElementById("message");
  if (!hash) {
    message.textContent = "Twitch did not return a token. You can close this window.";
  } else {
    fetch("/callback/token?" + hash)
      .then(function () { message.textContent = "Login complete — you can close this window."; })
      .catch(function () { message.textContent = "Something went wrong finishing login."; });
  }
</script>
</body>
</html>
`
