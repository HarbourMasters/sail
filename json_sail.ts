import Template from "https://deno.land/x/template@v0.1.0/mod.ts";
import { nanoid } from "https://deno.land/x/nanoid@v3.0.0/mod.ts";
import { Effect, Packet } from "./types.ts";
import { Sail } from "./Sail.ts";

let config: {
  channel: string;
  commands: {
    [index: string]: {
      cooldownSeconds?: number;
      lengthSeconds?: number;
      effects: Effect[];
      endEffects?: Effect[];
    };
  };
};

const sail = new Sail();
const tpl = new Template();
const onCooldown: Record<string, boolean> = {};

const configString = await Deno.readTextFileSync("./config.json");
if (!configString) {
  throw new Error("No config file found, please create a config.json");
}

try {
  config = JSON.parse(configString);
} catch (_error) {
  throw new Error("Failed to parse config.json");
}

sail.on("raw", (event) => {
  let command: string;
  let args: string[];
  if (event.raw.tags?.customRewardId) {
    command = event.raw.tags?.customRewardId;
    args = event.message.trim().split(" ");
  } else {
    if (!event.message) return;
    const splitMessage = event.message.trim().split(" ");
    command = splitMessage.shift()!;
    args = splitMessage;
  }

  if (config.commands[command]) {
    const commandConfig = config.commands[command];

    if (commandConfig.cooldownSeconds) {
      if (onCooldown[command]) {
        return;
      }
      onCooldown[command] = true;
    }

    const argObject = args!.reduce<Record<string, unknown>>(
      (result, arg, index) => {
        result[index] = arg;
        return result;
      },
      {},
    );
    const unformattedEffects = commandConfig.effects;
    const unformattedEndEffects = commandConfig.endEffects || [];
    const packets = preparePackets(unformattedEffects, argObject);
    const endPackets = preparePackets(
      unformattedEndEffects,
      argObject,
    );

    sail.queuePackets(packets);

    setTimeout(() => {
      sail.queuePackets(endPackets);
    }, (commandConfig.lengthSeconds || 0) * 1000);
    if (commandConfig.cooldownSeconds) {
      setTimeout(() => {
        onCooldown[command] = false;
      }, commandConfig.cooldownSeconds * 1000);
    }
  }
});

function preparePackets(effects: Effect[], argObject: any): Packet[] {
  return effects.map((effect) => {
    switch (effect.type) {
      case "apply":
        effect = {
          ...effect,
          parameters: effect.parameters?.map((param) => {
            if (typeof param === "string") {
              const result = parseInt(tpl.render(param, argObject));
              return isNaN(result) ? 0 : result;
            }
            return param;
          }),
        };
        break;
      case "command":
        effect = {
          ...effect,
          command: tpl.render(effect.command, argObject),
        };
        break;
      default:
    }

    return {
      id: nanoid(),
      type: "effect",
      effect,
    };
  });
}

sail.lift(config.channel)
  .catch((error) => {
    console.error(error);
    Deno.exit(1);
  });
