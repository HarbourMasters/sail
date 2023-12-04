import { Sail } from "../Sail.ts";
import { SohClient } from "../SohClient.ts";
import { TwitchClient } from "../TwitchClient.ts";

const sail = new Sail({ port: 43384, debug: true });
const twitchClient = new TwitchClient({ channel: "proxysaw" });
let sohClient: SohClient | undefined;

twitchClient.on("chat", (message, user) => {
  if (message.match(/!kick (\d+)/)) {
    if (onCooldown("kick", 1)) return;
    const strengthStr = message.match(/!kick (\d+)/)![1];
    const strength = Math.max(1, Math.min(3, parseInt(strengthStr)));
    sohClient?.knockbackPlayer(strength);
  }

  if (message.match(/!rave/)) {
    if (onCooldown("rave", 1)) return;

    Promise.all([
      sohClient?.command("set gCosmetics.Link_KokiriTunic.Changed 1"),
      sohClient?.command("set gCosmetics.Link_KokiriTunic.Rainbow 1"),
    ]).then(() => {
      setTimeout(() => {
        sohClient?.command("set gCosmetics.Link_KokiriTunic.Changed 0");
        sohClient?.command("set gCosmetics.Link_KokiriTunic.Rainbow 0");
      }, 1000 * 20);
    });
  }

  if (message.match(/!tiny/)) {
    if (onCooldown("tiny", 1)) return;
    sohClient?.modifyLinkSize(2, 10);
  }
});

twitchClient.on("redeem", (reward, message, user) => {
  if (reward === "878f54ca-b3ec-4acd-acc1-c5482b5c2f8e") {
    sohClient?.command("reset");
  }
});

twitchClient.on("bits", (bits, message, user) => {
});

sail.on("clientConnected", (client) => {
  sohClient = client;

  client.on("transitionEnd", ({ sceneNum }) => {
    console.log("OnTransitionEnd sceneNum:", sceneNum);
  });

  client.on("disconnected", () => {
    sohClient = undefined;
  });
});

const cooldownMap: Record<string, boolean> = {};
function onCooldown(command: string, cooldownSeconds: number) {
  if (!cooldownMap[command]) {
    cooldownMap[command] = true;
    setTimeout(() => {
      cooldownMap[command] = false;
    }, cooldownSeconds * 1000);
    return false;
  }

  return true;
}

(async () => {
  try {
    await twitchClient.connect();
    await sail.start();
  } catch (error) {
    console.error("There was an error starting the Custom Sail", error);
    Deno.exit(1);
  }
})();
