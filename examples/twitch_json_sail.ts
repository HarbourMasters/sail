import Template from "https://deno.land/x/template@v0.1.0/mod.ts";
import { nanoid } from "https://deno.land/x/nanoid@v3.0.0/mod.ts";
import { Effect, OutgoingPacket } from "../types.ts";
import { Sail } from "../Sail.ts";
import { TwitchClient } from "../TwitchClient.ts";
import { SohClient } from "../SohClient.ts";

let config: {
  port?: number;
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

const tpl = new Template();
const onCooldown: Record<string, boolean> = {};
let sohClient: SohClient | undefined;

const configString = await Deno.readTextFileSync("./config.json");
if (!configString) {
  throw new Error("No config file found, please create a config.json");
}

try {
  config = JSON.parse(configString);
} catch (_error) {
  throw new Error("Failed to parse config.json");
}

const sail = new Sail({ port: config.port || 43384, debug: true });
const twitchClient = new TwitchClient({ channel: config.channel });

sail.on("clientConnected", (client) => {
  sohClient = client;

  client.on("disconnected", () => {
    sohClient = undefined;
  });
});

twitchClient.on("raw", (event) => {
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

    Promise.all(packets.map((packet) => sohClient?.sendPacket(packet)))
      .then(() => {
        setTimeout(() => {
          endPackets.map((packet) => sohClient?.sendPacket(packet));
        }, (commandConfig.lengthSeconds || 0) * 1000);
      });

    if (commandConfig.cooldownSeconds) {
      setTimeout(() => {
        onCooldown[command] = false;
      }, commandConfig.cooldownSeconds * 1000);
    }
  }
});

function preparePackets(effects: Effect[], argObject: any): OutgoingPacket[] {
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

(async () => {
  try {
    await twitchClient.connect();
    await sail.start();
  } catch (error) {
    console.error("There was an error starting the JSON Twitch Sail", error);
    Deno.exit(1);
  }
})();
