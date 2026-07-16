import { useEffect, useState } from "react";
import {
  DeleteHookBinding,
  ListHookBindings,
  ListHooks,
  SaveHookBinding,
  UnwatchHook,
  WatchHook,
} from "../../wailsjs/go/main/App";
import { config, sail } from "../../wailsjs/go/models";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import BindingEditor from "../components/BindingEditor";
import { BindingDraft, bindingToDraft, bindingToPayload, describeBinding, emptyBinding } from "../lib/drafts";
import { HookEvent } from "../lib/events";
import { hookFields, hookTrigger } from "../lib/template";

interface HooksProps {
  actions: sail.ActionInfo[];
}

const MAX_FEED = 10;

export default function Hooks({ actions }: HooksProps) {
  const [catalog, setCatalog] = useState<sail.HookInfo[]>([]);
  const [bindings, setBindings] = useState<config.HookBinding[]>([]);
  const [recentHooks, setRecentHooks] = useState<HookEvent[]>([]);

  const [editingId, setEditingId] = useState<string | null>(null);
  const [hookName, setHookName] = useState("");
  const [idFilter, setIdFilter] = useState("");
  const [binding, setBinding] = useState<BindingDraft>(emptyBinding());
  const [error, setError] = useState("");
  // Hook currently listened for, or null. Listening is what subscribes the game to it.
  const [listeningHook, setListeningHook] = useState<string | null>(null);

  function refresh() {
    ListHookBindings().then(setBindings);
  }

  useEffect(() => {
    refresh();
    ListHooks().then(setCatalog);
    // Catalog upgrades from the compiled-in default to the game's live hook.list on connect.
    const offCatalog = EventsOn("hooks:updated", (updated: sail.HookInfo[]) => setCatalog(updated));
    const offHooks = EventsOn("sail:hook", (hook: HookEvent) =>
      setRecentHooks((prev) => [hook, ...prev].slice(0, MAX_FEED)),
    );
    return () => {
      offCatalog();
      offHooks();
    };
  }, []);

  // Subscribe the game to this hook while listening; unsubscribe on stop/unmount
  // so we never leave an expensive subscription running.
  useEffect(() => {
    if (!listeningHook) return;
    WatchHook(listeningHook);
    return () => {
      UnwatchHook(listeningHook);
    };
  }, [listeningHook]);

  const selectedHook = catalog.find((h) => h.name === hookName);
  const filterField = selectedHook?.filterField ?? "";
  const feedRows = listeningHook ? recentHooks.filter((h) => h.type === listeningHook) : [];

  function startNew() {
    setEditingId(null);
    setHookName("");
    setIdFilter("");
    setBinding(emptyBinding());
    setError("");
    setListeningHook(null);
  }

  function edit(hb: config.HookBinding) {
    setEditingId(hb.id);
    setHookName(hb.hookName);
    setIdFilter(hb.idFilter != null ? String(hb.idFilter) : "");
    setBinding(bindingToDraft(hb.binding));
    setError("");
    setListeningHook(null);
  }

  function selectHook(name: string) {
    setHookName(name);
    // Drop a stale id if the new hook can't be id-filtered.
    if (!catalog.find((h) => h.name === name)?.filterField) setIdFilter("");
    setListeningHook(null); // was listening for the previous hook
  }

  // Listen for the selected hook to read its ids off the feed. One at a time.
  function toggleListen() {
    if (listeningHook) {
      setListeningHook(null);
      return;
    }
    if (!hookName) return;
    setRecentHooks([]);
    setListeningHook(hookName);
  }

  async function save() {
    if (!hookName) {
      setError("Choose a hook to react to.");
      return;
    }

    let filterValue: number | undefined;
    if (filterField && idFilter.trim() !== "") {
      const parsed = Number(idFilter);
      if (!Number.isInteger(parsed)) {
        setError("The id filter must be a whole number, or left blank for any.");
        return;
      }
      filterValue = parsed;
    }

    try {
      await SaveHookBinding(
        config.HookBinding.createFrom({
          id: editingId ?? "",
          hookName,
          idFilter: filterValue,
          binding: bindingToPayload(binding, actions),
        }),
      );
      startNew();
      refresh();
    } catch (err) {
      setError(String(err));
    }
  }

  async function remove(id: string) {
    await DeleteHookBinding(id);
    if (editingId === id) startNew();
    refresh();
  }

  return (
    <div className="page">
      <h1>Game hooks</h1>
      <p className="page-hint">
        Fire effects from what happens in-game — an item pickup, a scene load, a flag flip — instead of from Twitch.
      </p>

      <div className="two-column">
        <div className="card list-card">
          {bindings.length === 0 && (
            <p className="empty-hint">Nothing bound to a hook yet — pick one on the right to react to an in-game event.</p>
          )}
          <ul className="config-list">
            {bindings.map((hb) => (
              <li key={hb.id} className={hb.id === editingId ? "active" : ""}>
                <button type="button" className="list-item-button" onClick={() => edit(hb)}>
                  <strong>
                    {hb.hookName}
                    {hb.idFilter != null && <span className="hook-filter-tag">id {hb.idFilter}</span>}
                  </strong>
                  <span className="list-item-detail">{describeBinding(hb.binding)}</span>
                </button>
                <button type="button" className="btn-icon" onClick={() => remove(hb.id)}>
                  Delete
                </button>
              </li>
            ))}
          </ul>
          <button type="button" className="btn-link" onClick={startNew}>
            + New hook binding
          </button>
        </div>

        <div className="card">
          <label className="trigger-input">
            Hook
            <select value={hookName} onChange={(e) => selectHook(e.target.value)}>
              <option value="" disabled>
                Choose a hook…
              </option>
              {catalog.map((hook) => (
                <option key={hook.name} value={hook.name}>
                  {hook.name}
                </option>
              ))}
            </select>
          </label>

          {filterField && (
            <label className="trigger-input">
              <span>
                Only when <code>{filterField}</code> equals
              </span>
              <input
                type="number"
                placeholder="any — fires on every one"
                value={idFilter}
                onChange={(e) => setIdFilter(e.target.value)}
              />
            </label>
          )}

          <BindingEditor
            binding={binding}
            actions={actions}
            trigger={hookTrigger(hookName)}
            onChange={setBinding}
          />

          {error && <div className="banner banner-error">{error}</div>}
          <button type="button" onClick={save}>
            {editingId ? "Save changes" : "Create hook binding"}
          </button>
        </div>
      </div>

      <div className="card">
        <div className="hook-feed-head">
          <h2>Live hook feed</h2>
          <button
            type="button"
            className={listeningHook ? "btn-ghost" : ""}
            disabled={!listeningHook && !hookName}
            onClick={toggleListen}
          >
            {listeningHook ? "Stop listening" : hookName ? `Listen for ${hookName}` : "Pick a hook to listen"}
          </button>
        </div>

        {!listeningHook ? (
          <p className="empty-hint">
            Only hooks with a binding are subscribed by default. Pick a hook above and listen to read its ids as it
            fires in-game — without the game streaming every actor spawn and flag flip.
          </p>
        ) : feedRows.length === 0 ? (
          <p className="empty-hint">
            Listening for <code>{listeningHook}</code> — trigger it in-game to see its ids here.
          </p>
        ) : (
          <ul className="hook-feed">
            {feedRows.map((hook, index) => (
              <li key={index} className="hook-feed-row">
                <code>{hook.type}</code>
                {hookFields(hook.type).map((field) => (
                  <span key={field} className="hook-feed-field">
                    {field} <strong>{String(hook[field as keyof HookEvent])}</strong>
                  </span>
                ))}
                {filterField && (
                  <button
                    type="button"
                    className="hook-feed-bind"
                    onClick={() => setIdFilter(String(hook[filterField as keyof HookEvent]))}
                  >
                    Use id →
                  </button>
                )}
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  );
}
