export interface CommandEffect {
  type: "command";
  command: string;
}

export interface ApplyEffect {
  type: "apply";
  name: string;
  parameters?: (number | string)[];
}

export interface RemoveEffect {
  type: "remove";
  name: string;
}

export type Effect = CommandEffect | ApplyEffect | RemoveEffect;

export interface EffectPacket {
  id: string;
  type: "effect";
  effect: Effect;
}

export interface CommandPacket {
  id: string;
  type: "command";
  command: string;
}

export interface ResultPacket {
  id: string;
  type: "result";
  status: "success" | "failure" | "try_again";
}

interface OnTransitionEndHook {
  type: "OnTransitionEnd";
  sceneNum: number;
}

interface OnLoadGameHook {
  type: "OnLoadGame";
  fileNum: number;
}

interface OnExitGameHook {
  type: "OnExitGame";
  fileNum: number;
}

interface OnItemReceiveHook {
  type: "OnItemReceive";
  tableId: number;
  getItemId: number;
}

interface OnEnemyDefeatHook {
  type: "OnEnemyDefeat";
  actorId: number;
  params: number;
}

interface OnActorInitHook {
  type: "OnActorInit";
  actorId: number;
  params: number;
}

interface OnFlagSetHook {
  type: "OnFlagSet";
  flagType: number;
  flag: number;
}

interface OnFlagUnsetHook {
  type: "OnFlagUnset";
  flagType: number;
  flag: number;
}

interface OnSceneFlagSetHook {
  type: "OnSceneFlagSet";
  flagType: number;
  flag: number;
  sceneNum: number;
}

interface OnSceneFlagUnsetHook {
  type: "OnSceneFlagUnset";
  flagType: number;
  flag: number;
  sceneNum: number;
}

export interface HookPacket {
  id: string;
  type: "hook";
  hook:
    | OnTransitionEndHook
    | OnLoadGameHook
    | OnExitGameHook
    | OnItemReceiveHook
    | OnEnemyDefeatHook
    | OnActorInitHook
    | OnFlagSetHook
    | OnFlagUnsetHook
    | OnSceneFlagSetHook
    | OnSceneFlagUnsetHook;
}

export type IncomingPacket = ResultPacket | HookPacket;
export type OutgoingPacket = EffectPacket | CommandPacket;
export type Packet = IncomingPacket | OutgoingPacket;
