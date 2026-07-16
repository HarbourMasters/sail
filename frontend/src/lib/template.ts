// {{...}} variables a chat/redeem trigger exposes to a binding's params and
// commands. Resolved on the Go side (dispatch.go) when the binding fires; the
// frontend only recognizes them (to keep templated values strings) and offers
// them in the editor.

export interface TemplateVariable {
  token: string;
  label: string;
}

// Keep this list in sync with the variables triggerContext.expand handles in
// dispatch.go.
export const TEMPLATE_VARIABLES: TemplateVariable[] = [
  { token: "{{message}}", label: "everything the viewer typed" },
  { token: "{{user}}", label: "who triggered it" },
  { token: "{{0}}", label: "just the first word ({{1}}, {{2}}, … for the rest)" },
];

// Detects a {{...}} reference; a value holding one is kept a string to the Go
// side (see paramValueForType), since its real value isn't known until fire.
const TEMPLATE_PATTERN = /\{\{\w+\}\}/;

export function containsTemplate(value: string): boolean {
  return TEMPLATE_PATTERN.test(value);
}

// ── Hook triggers ──────────────────────────────────────────────────────
// Fields each hook populates, mirroring the game's Sail.cpp handlers — a chip
// is offered only for a field the hook carries (Go resolves any of them
// regardless; see hookVariable in dispatch.go).
const HOOK_FIELDS: Record<string, string[]> = {
  OnSceneInit: ["sceneId"],
  OnItemGive: ["itemId"],
  OnActorInit: ["actorId", "params"],
  OnFlagSet: ["flagType", "flag"],
  OnFlagUnset: ["flagType", "flag"],
  OnSceneFlagSet: ["sceneId", "flagType", "flag"],
  OnSceneFlagUnset: ["sceneId", "flagType", "flag"],
};

const FIELD_LABELS: Record<string, string> = {
  sceneId: "the scene id",
  itemId: "the item id",
  actorId: "the actor id",
  params: "the actor's params",
  flagType: "the flag type",
  flag: "the flag id",
};

// Bare field list a hook populates, in editor order; empty for an unknown hook.
export function hookFields(hookName: string): string[] {
  return HOOK_FIELDS[hookName] ?? [];
}

// The {{...}} variables offered for a hook — hookFields as tokens with labels.
export function hookVariables(hookName: string): TemplateVariable[] {
  return hookFields(hookName).map((field) => ({
    token: `{{${field}}}`,
    label: FIELD_LABELS[field] ?? field,
  }));
}

// ── Trigger descriptors ─────────────────────────────────────────────────
// Everything the editor needs to describe a trigger: the {{...}} chips, the
// script `trigger.` accessor names, and a starter placeholder. Bundled so the
// chips and the inline hint can't drift — both come from one source.
export interface TriggerDescriptor {
  variables: TemplateVariable[];
  accessors: string[];
  placeholder: string;
}

// CHAT_TRIGGER is what a chat command or channel-point redeem exposes.
export const CHAT_TRIGGER: TriggerDescriptor = {
  variables: TEMPLATE_VARIABLES,
  accessors: ["user", "message", "args"],
  placeholder: `// trigger.user, trigger.message, trigger.args[]
// sail.action(name, params?, { duration?, lifetime? })
// sail.command(str)  sail.remove(name)  sail.chat(str)

if (trigger.args[0] === "heavy") {
  sail.action("gravity", { level: 3 }, { duration: 30 });
} else {
  sail.chat("no heavy today, " + trigger.user);
}`,
};

// hookTrigger is what a given game hook exposes, built from its field list so
// the chips, the script hint, and the placeholder can't disagree.
export function hookTrigger(hookName: string): TriggerDescriptor {
  const fields = hookFields(hookName);
  const example = fields[0]
    ? `if (trigger.${fields[0]} === 4) {\n  sail.action("gravity", { level: 3 }, { duration: 30 });\n}`
    : `sail.chat("something happened!");`;
  return {
    variables: hookVariables(hookName),
    accessors: ["hook", ...fields],
    placeholder: `// trigger.hook${fields[0] ? `, trigger.${fields[0]}` : ""}, …
// sail.action(name, params?, { duration?, lifetime? })
// sail.command(str)  sail.remove(name)  sail.chat(str)

${example}`,
  };
}
