import { useRef, useState } from "react";
import { sail } from "../../wailsjs/go/models";
import { emptyStep, FRAMES_PER_SECOND, StepDraft } from "../lib/drafts";
import { containsTemplate, TEMPLATE_VARIABLES, TemplateVariable } from "../lib/template";

interface StepListEditorProps {
  title: string;
  steps: StepDraft[];
  actions: sail.ActionInfo[];
  onChange: (steps: StepDraft[]) => void;
  // The {{...}} variables this trigger offers; defaults to the chat/redeem set.
  variables?: TemplateVariable[];
}

// Where a variable chip inserts: a step's command, its message, or an action param.
type TemplateTarget =
  | { kind: "command"; index: number }
  | { kind: "message"; index: number }
  | { kind: "param"; index: number; param: string };

// Edits a binding's Steps. Actions come from the game's live catalog (empty
// until a game connects). An int/float/bool param can't hold "{{message}}", so
// it carries a toggle that swaps in a text field for entering a variable.
export default function StepListEditor({ title, steps, actions, onChange, variables = TEMPLATE_VARIABLES }: StepListEditorProps) {
  // Params flipped into "type a variable" mode. A value already holding {{...}}
  // renders as text on its own, so this only covers the still-empty case.
  const [templateModeKeys, setTemplateModeKeys] = useState<Set<string>>(new Set());

  // Last focused template field, so a chip knows where to insert. A ref, not
  // state — focus tracking shouldn't re-render.
  const activeField = useRef<{ el: HTMLInputElement; target: TemplateTarget } | null>(null);

  function update(index: number, patch: Partial<StepDraft>) {
    onChange(steps.map((step, i) => (i === index ? { ...step, ...patch } : step)));
  }

  function updateParam(index: number, paramName: string, value: string) {
    update(index, { params: { ...steps[index].params, [paramName]: value } });
  }

  function selectAction(index: number, name: string) {
    // Params/duration/lifetime don't carry to a different action's schema.
    update(index, { name, params: {}, durationSeconds: "", lifetime: "session" });
  }

  const paramKey = (index: number, param: string) => `${index}:${param}`;

  function templateModeFor(index: number, param: string, value: string) {
    return templateModeKeys.has(paramKey(index, param)) || containsTemplate(value);
  }

  function enableTemplateMode(index: number, param: string) {
    setTemplateModeKeys((keys) => new Set(keys).add(paramKey(index, param)));
  }

  function disableTemplateMode(index: number, param: string) {
    setTemplateModeKeys((keys) => {
      const next = new Set(keys);
      next.delete(paramKey(index, param));
      return next;
    });
    // A leftover {{...}} would force the field back to template mode, so clear
    // it; a plain value the typed widget can show is kept.
    if (containsTemplate(steps[index].params[param] ?? "")) {
      updateParam(index, param, "");
    }
  }

  function trackFocus(el: HTMLInputElement, target: TemplateTarget) {
    activeField.current = { el, target };
  }

  function insertVariable(token: string) {
    const active = activeField.current;
    if (!active || !active.el.isConnected) return;

    const { el, target } = active;
    const start = el.selectionStart ?? el.value.length;
    const end = el.selectionEnd ?? el.value.length;
    const value = el.value.slice(0, start) + token + el.value.slice(end);

    if (target.kind === "command") {
      update(target.index, { command: value });
    } else if (target.kind === "message") {
      update(target.index, { message: value });
    } else {
      updateParam(target.index, target.param, value);
    }
  }

  return (
    <div className="step-list">
      <div className="step-list-header">
        <span>{title}</span>
        <button type="button" className="btn-link" onClick={() => onChange([...steps, emptyStep()])}>
          + Add step
        </button>
      </div>

      {steps.length === 0 && <p className="empty-hint">No steps yet.</p>}
      {actions.length === 0 && (
        <p className="empty-hint">No actions loaded yet — connect the game once to fetch its catalog.</p>
      )}

      {steps.map((step, index) => {
        const action = actions.find((a) => a.name === step.name);

        return (
          <div className="step-row" key={index}>
            <select value={step.kind} onChange={(e) => update(index, { kind: e.target.value as StepDraft["kind"] })}>
              <option value="action">Start action</option>
              <option value="remove">Cancel action</option>
              <option value="command">Run command</option>
              <option value="chat">Post chat message</option>
            </select>

            {step.kind === "command" ? (
              <input
                className="command-input"
                placeholder="set gCosmetics.Link_KokiriTunic.Changed 1"
                value={step.command}
                onFocus={(e) => trackFocus(e.currentTarget, { kind: "command", index })}
                onChange={(e) => update(index, { command: e.target.value })}
              />
            ) : step.kind === "chat" ? (
              <input
                className="command-input"
                placeholder="Thanks {{user}}!"
                value={step.message}
                onFocus={(e) => trackFocus(e.currentTarget, { kind: "message", index })}
                onChange={(e) => update(index, { message: e.target.value })}
              />
            ) : (
              <select value={step.name} onChange={(e) => selectAction(index, e.target.value)}>
                <option value="" disabled>
                  Choose an action…
                </option>
                {!action && step.name && <option value={step.name}>{step.name} (not in current catalog)</option>}
                {actions.map((a) => (
                  <option value={a.name} key={a.name}>
                    {a.displayName}
                  </option>
                ))}
              </select>
            )}

            <button type="button" className="btn-icon" onClick={() => onChange(steps.filter((_, i) => i !== index))}>
              Remove
            </button>

            {step.kind === "action" && action && (
              <div className="params">
                {action.params.map((spec) => {
                  const value = step.params[spec.name] ?? "";
                  const templated = templateModeFor(index, spec.name, value);
                  const target: TemplateTarget = { kind: "param", index, param: spec.name };

                  return (
                    <label className="param" key={spec.name}>
                      {spec.name}
                      <span className="param-value">
                        {templated ? (
                          <input
                            className="template-input"
                            placeholder="{{message}}"
                            value={value}
                            onFocus={(e) => trackFocus(e.currentTarget, target)}
                            onChange={(e) => updateParam(index, spec.name, e.target.value)}
                          />
                        ) : spec.type === "bool" ? (
                          <input
                            type="checkbox"
                            checked={value === "true"}
                            onChange={(e) => updateParam(index, spec.name, e.target.checked ? "true" : "false")}
                          />
                        ) : spec.type === "int" || spec.type === "float" ? (
                          <input
                            type="number"
                            step={spec.type === "float" ? "any" : "1"}
                            min={spec.min}
                            max={spec.max}
                            placeholder={spec.default != null ? String(spec.default) : undefined}
                            value={value}
                            onChange={(e) => updateParam(index, spec.name, e.target.value)}
                          />
                        ) : (
                          <input
                            placeholder={spec.default != null ? String(spec.default) : "text or {{message}}"}
                            value={value}
                            onFocus={(e) => trackFocus(e.currentTarget, target)}
                            onChange={(e) => updateParam(index, spec.name, e.target.value)}
                          />
                        )}

                        {spec.type !== "string" && (
                          <button
                            type="button"
                            className="btn-template-toggle"
                            aria-pressed={templated}
                            title={templated ? "Use a typed value instead" : "Use a variable like {{message}}"}
                            onClick={() =>
                              templated ? disableTemplateMode(index, spec.name) : enableTemplateMode(index, spec.name)
                            }
                          >
                            {templated ? "123" : "{ }"}
                          </button>
                        )}
                      </span>
                    </label>
                  );
                })}

                {action.timed && (
                  <>
                    <label className="param">
                      Duration (seconds)
                      <input
                        type="number"
                        min="0"
                        placeholder={String(action.defaultDuration / FRAMES_PER_SECOND)}
                        value={step.durationSeconds}
                        onChange={(e) => update(index, { durationSeconds: e.target.value })}
                      />
                    </label>
                    <label className="param">
                      Lifetime
                      <select
                        value={step.lifetime}
                        onChange={(e) => update(index, { lifetime: e.target.value as StepDraft["lifetime"] })}
                      >
                        <option value="session">Session (survives save loads)</option>
                        <option value="save">Save (dropped on file select/load)</option>
                      </select>
                    </label>
                  </>
                )}
              </div>
            )}
          </div>
        );
      })}

      {variables.length > 0 && (
        <div className="template-legend">
          <span className="template-legend-title">Variables</span>
          <div className="template-legend-chips">
            {variables.map((variable) => (
              <button
                type="button"
                key={variable.token}
                className="chip"
                title={`Insert ${variable.token}`}
                // Keep the focused field focused so the insert lands there.
                onMouseDown={(e) => e.preventDefault()}
                onClick={() => insertVariable(variable.token)}
              >
                <code>{variable.token}</code>
                <span>{variable.label}</span>
              </button>
            ))}
          </div>
          <p className="template-legend-hint">
            Click a command or param field, then a variable to drop it in — or just type it.
          </p>
        </div>
      )}
    </div>
  );
}
