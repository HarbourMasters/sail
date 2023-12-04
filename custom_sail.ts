import { Sail } from "./Sail.ts";
import { TwitchClient } from "./TwitchClient.ts";

const sail = new Sail();
const twitchClient = new TwitchClient();

twitchClient.on("chat", (message, user) => {
  if (message.match(/!kick (\d+)/)) {
    if (onCooldown("kick", 1)) return;
    const strengthStr = message.match(/!kick (\d+)/)![1];
    const strength = Math.max(1, Math.min(3, parseInt(strengthStr)));
    sail.knockbackPlayer(strength);
  }

  if (message.match(/!rave/)) {
    if (onCooldown("rave", 1)) return;
    sail.command("set gCosmetics.Link_KokiriTunic.Changed 1");
    sail.command("set gCosmetics.Link_KokiriTunic.Rainbow 1");

    setTimeout(() => {
      sail.command("set gCosmetics.Link_KokiriTunic.Changed 0");
      sail.command("set gCosmetics.Link_KokiriTunic.Rainbow 0");
    }, 1000 * 20);
  }

  if (message.match(/!tiny/)) {
    if (onCooldown("tiny", 1)) return;
    sail.modifyLinkSize(2, 10);
  }
});

twitchClient.on("redeem", (reward, message, user) => {
  if (reward === "878f54ca-b3ec-4acd-acc1-c5482b5c2f8e") {
    sail.command("reset");
  }
});

twitchClient.on("bits", (bits, message, user) => {
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
    await twitchClient.connect("proxysaw");
    await sail.lift();
  } catch (error) {
    console.error("There was an error starting the Custom Sail", error);
    Deno.exit(1);
  }
})();
