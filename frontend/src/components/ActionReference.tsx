import { useEffect, useState } from "react";
import { sail } from "../../wailsjs/go/models";
import { ClipboardSetText } from "../../wailsjs/runtime/runtime";
import { FRAMES_PER_SECOND } from "../lib/drafts";

interface ActionReferenceProps {
  actions: sail.ActionInfo[];
  onClose: () => void;
}

// Wails clipboard inside the webview; browser API as a fallback (e.g. running
// the frontend under plain `vite`).
async function copyText(text: string) {
  try {
    await ClipboardSetText(text);
  } catch {
    try {
      await navigator.clipboard.writeText(text);
    } catch {
      /* copy silently fails */
    }
  }
}

// An illustrative literal for a param: its default, else a type-appropriate
// value (min for numbers, to stay in range).
function exampleValue(spec: sail.ParamSpec): string {
  if (spec.default != null) {
    return JSON.stringify(spec.default);
  }
  switch (spec.type) {
    case "bool":
      return "true";
    case "int":
      return spec.min != null ? String(spec.min) : "1";
    case "float":
      return spec.min != null ? String(spec.min) : "1.5";
    default:
      return '"text"';
  }
}

// A runnable sail.action(...) line, with a { duration } options object for
// timed actions.
function exampleCall(action: sail.ActionInfo): string {
  const name = JSON.stringify(action.name);
  const params = action.params.length ? `{ ${action.params.map((p) => `${p.name}: ${exampleValue(p)}`).join(", ")} }` : "";

  if (action.timed) {
    const seconds = action.defaultDuration / FRAMES_PER_SECOND;
    return `sail.action(${name}, ${params || "{}"}, { duration: ${seconds} });`;
  }
  return params ? `sail.action(${name}, ${params});` : `sail.action(${name});`;
}

function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false);

  async function copy() {
    await copyText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 1200);
  }

  return (
    <button type="button" className="copy-btn" onClick={copy}>
      {copied ? "Copied!" : "Copy"}
    </button>
  );
}

function paramRange(spec: sail.ParamSpec): string | null {
  if (spec.min == null && spec.max == null) return null;
  return `${spec.min ?? "…"}–${spec.max ?? "…"}`;
}

// Modal listing the live action catalog with copy-able sail.action(...)
// examples. Empty until a game connects (same catalog as the step picker).
export default function ActionReference({ actions, onClose }: ActionReferenceProps) {
  // Esc closes.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal" role="dialog" aria-modal="true" aria-label="Actions reference" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h2>Scripting reference</h2>
          <button type="button" className="btn-icon" onClick={onClose}>
            Close
          </button>
        </div>

        <div className="modal-body">
          <section className="doc-basics">
            <p className="doc-basics-line">
              <code>trigger.user</code> · <code>trigger.message</code> · <code>trigger.args</code> — who/what fired it.
            </p>
            <div className="example-row">
              <code className="example-code">sail.command("givehealth 20");</code>
              <CopyButton text={'sail.command("givehealth 20");'} />
            </div>
            <div className="example-row">
              <code className="example-code">sail.remove("gravity");</code>
              <CopyButton text={'sail.remove("gravity");'} />
            </div>
            <div className="example-row">
              <code className="example-code">sail.chat("Thanks " + trigger.user + "!");</code>
              <CopyButton text={'sail.chat("Thanks " + trigger.user + "!");'} />
            </div>
          </section>

          <h3 className="doc-actions-title">Actions</h3>

          {actions.length === 0 ? (
            <p className="empty-hint">No actions loaded yet — connect the game once to fetch its catalog.</p>
          ) : (
            actions.map((action) => {
              const example = exampleCall(action);
              return (
                <div className="action-doc" key={action.name}>
                  <div className="action-doc-head">
                    <code className="action-doc-name">{action.name}</code>
                    <span className="action-doc-display">{action.displayName}</span>
                    {action.timed && (
                      <span className="action-doc-badge">timed · {action.defaultDuration / FRAMES_PER_SECOND}s default</span>
                    )}
                    {action.valence && action.valence !== "neutral" && (
                      <span className={`action-doc-badge valence-${action.valence}`}>{action.valence}</span>
                    )}
                  </div>

                  {action.params.length === 0 ? (
                    <p className="param-doc-empty">No parameters.</p>
                  ) : (
                    <ul className="param-doc-list">
                      {action.params.map((spec) => {
                        const range = paramRange(spec);
                        return (
                          <li key={spec.name}>
                            <code className="param-doc-name">{spec.name}</code>
                            <span className="param-doc-type">{spec.type}</span>
                            {spec.required && <span className="param-doc-req">required</span>}
                            {spec.default != null && <span className="param-doc-meta">default {String(spec.default)}</span>}
                            {range && <span className="param-doc-meta">{range}</span>}
                          </li>
                        );
                      })}
                    </ul>
                  )}

                  <div className="example-row">
                    <code className="example-code">{example}</code>
                    <CopyButton text={example} />
                  </div>
                </div>
              );
            })
          )}
        </div>
      </div>
    </div>
  );
}
