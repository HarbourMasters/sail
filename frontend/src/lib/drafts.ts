import { config, sail } from "../../wailsjs/go/models";
import { containsTemplate } from "./template";

// Editing works against plain "draft" shapes (params always strings, as text
// inputs/checkboxes produce), converting to the generated Wails model classes
// only at the bound-method boundary. draft->payload needs the action catalog
// to type each param and read its defaultDuration; payload->draft doesn't, as
// a JSON value already carries its own JS type.

// Wire durations are in frames (20/sec); the editor works in seconds. Single
// source for the frontend.
export const FRAMES_PER_SECOND = 20;

export interface StepDraft {
  kind: "action" | "remove" | "command" | "chat";
  name: string; // action/remove: the chosen action's name
  params: Record<string, string>; // action: raw text/checkbox state per param name
  durationSeconds: string; // action: override the action's defaultDuration; blank = use it
  lifetime: "session" | "save"; // action
  command: string; // command: the raw string to run
  message: string; // chat: the message to post (may hold {{variables}})
}

export interface BindingDraft {
  steps: StepDraft[];
  script: string; // optional JS that produces the steps instead of the list above
  cooldownSeconds: string;
}

export function emptyStep(): StepDraft {
  return { kind: "action", name: "", params: {}, durationSeconds: "", lifetime: "session", command: "", message: "" };
}

export function emptyBinding(): BindingDraft {
  return { steps: [], script: "", cooldownSeconds: "" };
}

function stepToDraft(step: config.Step): StepDraft {
  const params: Record<string, string> = {};
  for (const [key, value] of Object.entries(step.params ?? {})) {
    params[key] = value == null ? "" : String(value);
  }

  return {
    kind: (step.kind as StepDraft["kind"]) || "action",
    name: step.name ?? "",
    params,
    durationSeconds: step.duration != null ? String(step.duration / FRAMES_PER_SECOND) : "",
    lifetime: (step.lifetime as StepDraft["lifetime"]) || "session",
    command: step.command ?? "",
    message: step.message ?? "",
  };
}

export function bindingToDraft(binding: config.Binding): BindingDraft {
  return {
    steps: (binding.steps ?? []).map(stepToDraft),
    script: binding.script ?? "",
    cooldownSeconds: binding.cooldownSeconds ? String(binding.cooldownSeconds) : "",
  };
}

// Converts one param's raw text (or "true"/"false") to the JSON type its
// schema declares. A {{...}} reference stays a string — it's resolved
// server-side when the binding fires, whatever type the param declares.
function paramValueForType(text: string, type: string): string | number | boolean {
  const trimmed = text.trim();
  if (containsTemplate(trimmed)) {
    return trimmed;
  }

  switch (type) {
    case "bool":
      return trimmed === "true";
    case "int": {
      const n = parseInt(trimmed, 10);
      return Number.isNaN(n) ? 0 : n;
    }
    case "float": {
      const n = parseFloat(trimmed);
      return Number.isNaN(n) ? 0 : n;
    }
    default:
      return trimmed;
  }
}

function stepToPayload(draft: StepDraft, actions: sail.ActionInfo[]): config.Step {
  if (draft.kind === "command") {
    return config.Step.createFrom({ kind: "command", command: draft.command });
  }

  if (draft.kind === "chat") {
    return config.Step.createFrom({ kind: "chat", message: draft.message });
  }

  if (draft.kind === "remove") {
    return config.Step.createFrom({ kind: "remove", name: draft.name.trim() });
  }

  const action = actions.find((a) => a.name === draft.name);
  const params: Record<string, string | number | boolean> = {};
  if (action) {
    for (const spec of action.params) {
      params[spec.name] = paramValueForType(draft.params[spec.name] ?? "", spec.type);
    }
  }

  const durationSeconds = draft.durationSeconds.trim() ? Number(draft.durationSeconds) : undefined;

  return config.Step.createFrom({
    kind: "action",
    name: draft.name.trim(),
    params: Object.keys(params).length ? params : undefined,
    duration: durationSeconds != null ? Math.round(durationSeconds * FRAMES_PER_SECOND) : undefined,
    lifetime: draft.lifetime !== "session" ? draft.lifetime : undefined,
  });
}

export function bindingToPayload(draft: BindingDraft, actions: sail.ActionInfo[]): config.Binding {
  return config.Binding.createFrom({
    steps: draft.steps.map((step) => stepToPayload(step, actions)),
    // Keep the author's exact text; only its emptiness marks the binding
    // scripted (matches fireBinding's TrimSpace).
    script: draft.script.trim() ? draft.script : undefined,
    cooldownSeconds: draft.cooldownSeconds.trim() ? Number(draft.cooldownSeconds) : 0,
  });
}

export function describeBinding(binding: config.Binding): string {
  // A scripted binding's steps come from the script, not the static list.
  if (binding.script && binding.script.trim()) return "custom script";

  const names = (binding.steps ?? []).map((step) => {
    if (step.kind === "command") return `run "${step.command}"`;
    if (step.kind === "chat") return `say "${step.message}"`;
    if (step.kind === "remove") return `cancel ${step.name}`;
    return step.name || "(no action selected)";
  });
  return names.length ? names.join(", ") : "no steps configured";
}
