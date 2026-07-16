import { useState } from "react";
import { sail } from "../../wailsjs/go/models";
import { BindingDraft } from "../lib/drafts";
import { CHAT_TRIGGER, TriggerDescriptor } from "../lib/template";
import ActionReference from "./ActionReference";
import StepListEditor from "./StepListEditor";

interface BindingEditorProps {
  binding: BindingDraft;
  actions: sail.ActionInfo[];
  onChange: (binding: BindingDraft) => void;
  // What this trigger exposes; defaults to the chat/redeem set.
  trigger?: TriggerDescriptor;
}

// A binding: a Steps list, plus an advanced Script section that — when
// non-empty — runs instead of the steps and emits them itself.
export default function BindingEditor({ binding, actions, onChange, trigger = CHAT_TRIGGER }: BindingEditorProps) {
  const scriptActive = binding.script.trim().length > 0;
  const [showReference, setShowReference] = useState(false);

  // Derived from the same accessors as the chips, so hint and chips can't drift.
  const scriptTrigger = (
    <>
      (
      {trigger.accessors.map((accessor, i) => (
        <span key={accessor}>
          {i > 0 ? ", " : ""}
          <code>.{accessor}</code>
        </span>
      ))}
      )
    </>
  );

  return (
    <div className="binding-editor">
      <StepListEditor
        title="Steps"
        steps={binding.steps}
        actions={actions}
        variables={trigger.variables}
        onChange={(steps) => onChange({ ...binding, steps })}
      />

      <details className="script-section" open={scriptActive}>
        <summary>
          Script <span className="script-tag">advanced</span>
        </summary>

        <p className="script-hint">
          JavaScript run when this fires. It gets <code>trigger</code> {scriptTrigger} and calls{" "}
          <code>sail.action</code>, <code>sail.command</code>, <code>sail.remove</code>, <code>sail.chat</code> to
          decide the steps. Durations are in seconds.{" "}
          <button type="button" className="btn-link" onClick={() => setShowReference(true)}>
            Actions reference
          </button>
        </p>

        {scriptActive && (
          <p className="banner banner-warning script-override-note">
            While this script has content it runs <strong>instead of</strong> the steps above. Clear it to go back to
            the step list.
          </p>
        )}

        <textarea
          className="script-input"
          spellCheck={false}
          rows={10}
          placeholder={trigger.placeholder}
          value={binding.script}
          onChange={(e) => onChange({ ...binding, script: e.target.value })}
        />
      </details>

      <div className="binding-timing">
        <label>
          Cooldown (seconds)
          <input
            type="number"
            min="0"
            value={binding.cooldownSeconds}
            onChange={(e) => onChange({ ...binding, cooldownSeconds: e.target.value })}
          />
        </label>
      </div>

      {showReference && <ActionReference actions={actions} onClose={() => setShowReference(false)} />}
    </div>
  );
}
