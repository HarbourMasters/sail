// Payload shapes for Go->frontend events that the generated wailsjs bindings
// don't cover (Wails only types bound-method params/returns), kept in sync with
// the Go structs by hand. HookEvent mirrors Hook in internal/sail/protocol.go.

export interface HookEvent {
  type: string;
  sceneId: number;
  itemId: number;
  actorId: number;
  params: number;
  flagType: number;
  flag: number;
}
