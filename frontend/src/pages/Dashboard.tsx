import { useEffect, useState } from "react";
import {
  GetRecentActivity,
  LoginWithTwitch,
  LogoutFromTwitch,
  StartServer,
  StopServer,
} from "../../wailsjs/go/main/App";
import { main } from "../../wailsjs/go/models";
import { EventsOn } from "../../wailsjs/runtime/runtime";

const MAX_ACTIVITY = 20;

const activityKey = (event: main.ActivityEvent) => `${event.at}|${event.source}|${event.user}|${event.trigger}`;

interface DashboardProps {
  // Owned by App (drives the sidebar too) and passed down — no second copy here.
  server: main.ServerStatus | null;
  twitch: main.TwitchStatus | null;
}

export default function Dashboard({ server, twitch }: DashboardProps) {
  const [activity, setActivity] = useState<main.ActivityEvent[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    // Subscribe before fetching the buffer so nothing fired in between is missed.
    const offActivity = EventsOn("activity", (event: main.ActivityEvent) => {
      setActivity((prev) => [event, ...prev].slice(0, MAX_ACTIVITY));
    });

    GetRecentActivity().then((recent) => {
      setActivity((live) => {
        // Keep anything already arrived live in front; append the buffer, minus dupes.
        if (live.length === 0) return recent;
        const seen = new Set(live.map(activityKey));
        return [...live, ...recent.filter((event) => !seen.has(activityKey(event)))].slice(0, MAX_ACTIVITY);
      });
    });

    return () => {
      offActivity();
    };
  }, []);

  async function toggleServer() {
    setBusy(true);
    setError("");
    try {
      await (server?.running ? StopServer() : StartServer());
    } catch (err) {
      setError(String(err));
    } finally {
      setBusy(false);
    }
  }

  async function toggleTwitch() {
    setBusy(true);
    setError("");
    try {
      await (twitch?.loggedIn ? LogoutFromTwitch() : LoginWithTwitch());
    } catch (err) {
      setError(String(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="page">
      <h1>Dashboard</h1>
      <p className="page-hint">Your control deck — connections at a glance and a running log of what chat sets off.</p>

      {twitch?.needsClientId && (
        <div className="banner banner-warning">
          No Twitch application is configured yet. See the Settings page for setup instructions.
        </div>
      )}
      {error && <div className="banner banner-error">{error}</div>}

      <div className="card-grid">
        <div className="card">
          <h2>Game connection</h2>
          <p className="status-line">
            <span className={`dot ${server?.running ? "dot-on" : "dot-off"}`} />
            {server?.running ? `Listening on port ${server.port}` : "Stopped"}
          </p>
          <p>{server?.connectedClients ?? 0} game client(s) connected</p>
          <button type="button" disabled={busy} onClick={toggleServer}>
            {server?.running ? "Stop server" : "Start server"}
          </button>
        </div>

        <div className="card">
          <h2>Twitch</h2>
          <p className="status-line">
            <span className={`dot ${twitch?.loggedIn ? "dot-on" : "dot-off"}`} />
            {twitch?.loggedIn ? `Logged in as ${twitch.login}` : "Not logged in"}
          </p>
          <button
            type="button"
            className="btn-twitch"
            disabled={busy || twitch?.needsClientId}
            onClick={toggleTwitch}
          >
            {twitch?.loggedIn ? "Log out" : "Log in with Twitch"}
          </button>
          {twitch?.eventSubError && (
            <p className="banner banner-error">Chat/redeem connection: {twitch.eventSubError}</p>
          )}
          {twitch?.loggedIn && !twitch.canPostChat && (
            <p className="banner banner-warning">
              To post to chat from a binding, log out and back in — it adds a chat-write permission.
            </p>
          )}
        </div>
      </div>

      <div className="card">
        <h2>Ship's log</h2>
        {activity.length === 0 ? (
          <p className="empty-hint">The log is clear. Chat commands and redemptions are recorded here as they fire.</p>
        ) : (
          <ul className="activity-list">
            {activity.map((event, index) => (
              <li key={index} className={event.error ? "activity-item-error" : ""}>
                <span className={`badge badge-${event.source}`}>{event.source}</span>
                {event.user ? (
                  <>
                    <strong>{event.user}</strong> triggered <code>{event.trigger}</code>
                  </>
                ) : (
                  <>
                    <code>{event.trigger}</code> fired
                  </>
                )}
                <span className="activity-time">{new Date(event.at).toLocaleTimeString()}</span>
                {event.error && <span className="activity-error-msg">Script error: {event.error}</span>}
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  );
}
