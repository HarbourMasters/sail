import { useEffect, useState } from "react";
import { DeleteCommand, ListCommands, SaveCommand } from "../../wailsjs/go/main/App";
import { config, sail } from "../../wailsjs/go/models";
import BindingEditor from "../components/BindingEditor";
import { BindingDraft, bindingToDraft, bindingToPayload, describeBinding, emptyBinding } from "../lib/drafts";

interface CommandsProps {
  actions: sail.ActionInfo[];
}

export default function Commands({ actions }: CommandsProps) {
  const [commands, setCommands] = useState<config.Command[]>([]);
  const [editingTrigger, setEditingTrigger] = useState<string | null>(null);
  const [trigger, setTrigger] = useState("");
  const [binding, setBinding] = useState<BindingDraft>(emptyBinding());
  const [error, setError] = useState("");

  function refresh() {
    ListCommands().then(setCommands);
  }

  useEffect(refresh, []);

  function startNew() {
    setEditingTrigger(null);
    setTrigger("");
    setBinding(emptyBinding());
    setError("");
  }

  function edit(command: config.Command) {
    setEditingTrigger(command.trigger);
    setTrigger(command.trigger);
    setBinding(bindingToDraft(command.binding));
    setError("");
  }

  async function save() {
    if (!trigger.trim()) {
      setError("Enter a trigger, e.g. !kick");
      return;
    }

    try {
      await SaveCommand(config.Command.createFrom({ trigger, binding: bindingToPayload(binding, actions) }));
      if (editingTrigger && editingTrigger !== trigger) {
        await DeleteCommand(editingTrigger);
      }
      startNew();
      refresh();
    } catch (err) {
      setError(String(err));
    }
  }

  async function remove(t: string) {
    await DeleteCommand(t);
    if (editingTrigger === t) startNew();
    refresh();
  }

  return (
    <div className="page">
      <h1>Chat commands</h1>
      <p className="page-hint">Trigger words viewers type in chat, e.g. "!kick 2", mapped to in-game effects.</p>

      <div className="two-column">
        <div className="card list-card">
          {commands.length === 0 && <p className="empty-hint">No commands charted yet — add one on the right.</p>}
          <ul className="config-list">
            {commands.map((command) => (
              <li key={command.trigger} className={command.trigger === editingTrigger ? "active" : ""}>
                <button type="button" className="list-item-button" onClick={() => edit(command)}>
                  <strong>{command.trigger}</strong>
                  <span className="list-item-detail">{describeBinding(command.binding)}</span>
                </button>
                <button type="button" className="btn-icon" onClick={() => remove(command.trigger)}>
                  Delete
                </button>
              </li>
            ))}
          </ul>
          <button type="button" className="btn-link" onClick={startNew}>
            + New command
          </button>
        </div>

        <div className="card">
          <label className="trigger-input">
            Trigger
            <input value={trigger} onChange={(e) => setTrigger(e.target.value)} placeholder="!kick" />
          </label>

          <BindingEditor binding={binding} actions={actions} onChange={setBinding} />

          {error && <div className="banner banner-error">{error}</div>}
          <button type="button" onClick={save}>
            {editingTrigger ? "Save changes" : "Create command"}
          </button>
        </div>
      </div>
    </div>
  );
}
