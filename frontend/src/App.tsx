import { ReactNode, useEffect, useState } from "react";
import "./App.css";
import { GetServerStatus, GetTwitchStatus, ListActions } from "../wailsjs/go/main/App";
import { main, sail } from "../wailsjs/go/models";
import { EventsOn } from "../wailsjs/runtime/runtime";
import ErrorBoundary from "./components/ErrorBoundary";
import Commands from "./pages/Commands";
import Dashboard from "./pages/Dashboard";
import Hooks from "./pages/Hooks";
import Redeems from "./pages/Redeems";
import Settings from "./pages/Settings";

type Page = "dashboard" | "commands" | "redeems" | "hooks" | "settings";

// Ship's-wheel brand mark.
function BrandWheel() {
  return (
    <svg className="brand-mark" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" aria-hidden="true">
      <circle cx="12" cy="12" r="7.5" />
      <circle cx="12" cy="12" r="2.4" />
      <line x1="12" y1="1.5" x2="12" y2="22.5" />
      <line x1="1.5" y1="12" x2="22.5" y2="12" />
      <line x1="4.58" y1="4.58" x2="19.42" y2="19.42" />
      <line x1="19.42" y1="4.58" x2="4.58" y2="19.42" />
    </svg>
  );
}

// Nav glyphs (Feather-style).
function IconCompass() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.9" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <circle cx="12" cy="12" r="9" />
      <path d="M15.6 8.4 11 11 8.4 15.6 13 13 Z" />
    </svg>
  );
}

function IconChat() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.9" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z" />
    </svg>
  );
}

function IconGift() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.9" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <polyline points="20 12 20 22 4 22 4 12" />
      <rect x="2" y="7" width="20" height="5" />
      <line x1="12" y1="22" x2="12" y2="7" />
      <path d="M12 7H7.5a2.5 2.5 0 0 1 0-5C11 2 12 7 12 7z" />
      <path d="M12 7h4.5a2.5 2.5 0 0 0 0-5C13 2 12 7 12 7z" />
    </svg>
  );
}

function IconSliders() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.9" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <line x1="4" y1="21" x2="4" y2="14" />
      <line x1="4" y1="10" x2="4" y2="3" />
      <line x1="12" y1="21" x2="12" y2="12" />
      <line x1="12" y1="8" x2="12" y2="3" />
      <line x1="20" y1="21" x2="20" y2="16" />
      <line x1="20" y1="12" x2="20" y2="3" />
      <line x1="1" y1="14" x2="7" y2="14" />
      <line x1="9" y1="8" x2="15" y2="8" />
      <line x1="17" y1="16" x2="23" y2="16" />
    </svg>
  );
}

function IconHook() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.9" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <circle cx="15" cy="4.4" r="1.7" />
      <path d="M15 6.1 V14 a5 5 0 1 1 -10 0" />
      <path d="M3.3 12.5 5 14.3 6.8 12.7" />
    </svg>
  );
}

const PAGES: { id: Page; label: string; icon: ReactNode }[] = [
  { id: "dashboard", label: "Dashboard", icon: <IconCompass /> },
  { id: "commands", label: "Commands", icon: <IconChat /> },
  { id: "redeems", label: "Redeems", icon: <IconGift /> },
  { id: "hooks", label: "Hooks", icon: <IconHook /> },
  { id: "settings", label: "Settings", icon: <IconSliders /> },
];

function App() {
  const [page, setPage] = useState<Page>("dashboard");
  const [actions, setActions] = useState<sail.ActionInfo[]>([]);
  const [server, setServer] = useState<main.ServerStatus | null>(null);
  const [twitch, setTwitch] = useState<main.TwitchStatus | null>(null);

  useEffect(() => {
    ListActions().then(setActions);
    // "actions:updated" fires once the Go side has the fresh catalog — after
    // "server:status", which would otherwise race ahead of it.
    return EventsOn("actions:updated", (updated: sail.ActionInfo[]) => setActions(updated));
  }, []);

  // Status lives here (not just the Dashboard) so the sidebar shows it on
  // every page.
  useEffect(() => {
    GetServerStatus().then(setServer);
    GetTwitchStatus().then(setTwitch);
    const offServer = EventsOn("server:status", (status: main.ServerStatus) => setServer(status));
    const offTwitch = EventsOn("twitch:status", (status: main.TwitchStatus) => setTwitch(status));
    return () => {
      offServer();
      offTwitch();
    };
  }, []);

  return (
    <div id="app">
      <nav className="sidebar">
        <div className="brand">
          <BrandWheel />
          <span>SAIL</span>
        </div>
        <div className="wave-divider" />

        <div className="nav">
          {PAGES.map((item) => (
            <button
              key={item.id}
              type="button"
              className={item.id === page ? "nav-item active" : "nav-item"}
              onClick={() => setPage(item.id)}
            >
              {item.icon}
              {item.label}
            </button>
          ))}
        </div>

        <div className="sidebar-status">
          <div className="status-chip">
            <span className={`dot ${server?.running ? "dot-on" : "dot-off"}`} />
            <span className="status-chip-label">Game</span>
            {server?.running && (
              <span className="status-chip-note">
                {server.connectedClients > 0 ? `${server.connectedClients} aboard` : "waiting"}
              </span>
            )}
          </div>
          <div className="status-chip">
            <span className={`dot ${twitch?.loggedIn ? "dot-on" : "dot-off"}`} />
            <span className="status-chip-label">Twitch</span>
            {twitch?.loggedIn && twitch.login && <span className="status-chip-note">{twitch.login}</span>}
          </div>
        </div>
      </nav>

      <main className="content">
        <ErrorBoundary key={page}>
          {page === "dashboard" && <Dashboard server={server} twitch={twitch} />}
          {page === "commands" && <Commands actions={actions} />}
          {page === "redeems" && <Redeems actions={actions} />}
          {page === "hooks" && <Hooks actions={actions} />}
          {page === "settings" && <Settings twitch={twitch} />}
        </ErrorBoundary>
      </main>
    </div>
  );
}

export default App;
