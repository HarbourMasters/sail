export interface ConsoleEffect {
  type: "console";
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

export type Effect = ConsoleEffect | ApplyEffect | RemoveEffect;

export interface Packet {
  id: string;
  effect: Effect;
  status?: "success" | "error" | "try_again";
}
