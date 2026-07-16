import { useEffect, useState } from "react";
import { GetServerStatus, LogoutFromTwitch, SetPort } from "../../wailsjs/go/main/App";
import { main } from "../../wailsjs/go/models";

interface SettingsProps {
  // Owned by App and passed down so it stays live here.
  twitch: main.TwitchStatus | null;
}

export default function Settings({ twitch }: SettingsProps) {
  const [port, setPortValue] = useState("");
  const [error, setError] = useState("");
  const [saved, setSaved] = useState(false);

  useEffect(() => {
    GetServerStatus().then((status) => setPortValue(String(status.port)));
  }, []);

  async function savePort() {
    setError("");
    setSaved(false);
    const parsed = Number(port);
    if (!Number.isInteger(parsed) || parsed < 1 || parsed > 65535) {
      setError("Enter a port between 1 and 65535");
      return;
    }
    try {
      await SetPort(parsed);
      setSaved(true);
    } catch (err) {
      setError(String(err));
    }
  }

  async function logout() {
    await LogoutFromTwitch();
  }

  return (
    <div className="page">
      <h1>Settings</h1>
      <p className="page-hint">The port your game connects on, your Twitch account, and one-time setup.</p>

      <div className="card">
        <h2>Game server</h2>
        <p className="page-hint">
          The port the Sail-enabled Ship of Harkinian build connects to. Changing it restarts the server if it's
          currently running.
        </p>
        <label>
          Port
          <input
            value={port}
            onChange={(e) => {
              setPortValue(e.target.value);
              setSaved(false);
            }}
          />
        </label>
        <button type="button" onClick={savePort}>
          Save port
        </button>
        {saved && <span className="inline-success"> Saved.</span>}
        {error && <div className="banner banner-error">{error}</div>}
      </div>

      <div className="card">
        <h2>Twitch account</h2>
        {twitch?.loggedIn ? (
          <>
            <p>
              Logged in as <strong>{twitch.login}</strong>.
            </p>
            <button type="button" className="btn-twitch" onClick={logout}>
              Log out
            </button>
          </>
        ) : (
          <p>Not logged in. Use the Dashboard to log in once a Twitch application is configured below.</p>
        )}
      </div>

      {twitch?.needsClientId && (
        <div className="card">
          <h2>Register a Twitch application</h2>
          <p>This build doesn't have a Twitch Client ID yet. To enable login:</p>
          <ol>
            <li>
              Go to the Twitch Developer Console at <code>https://dev.twitch.tv/console/apps</code> and register a
              new application.
            </li>
            <li>
              Set <strong>OAuth Redirect URL</strong> to exactly <code>http://localhost:43385/callback</code>.
            </li>
            <li>
              Set <strong>Client Type</strong> to <strong>Public</strong>.
            </li>
            <li>
              Copy the Client ID and either set it as the <code>SAIL_TWITCH_CLIENT_ID</code> environment variable
              before launching Sail, or replace the placeholder in{" "}
              <code>internal/twitchapi/client_id.go</code> and rebuild.
            </li>
          </ol>
        </div>
      )}
    </div>
  );
}
