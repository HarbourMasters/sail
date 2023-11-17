import { Sail } from "./Sail.ts";

const sail = new Sail();

sail.on("chat", (message, user) => {
  if (message.match(/!kick (\d+)/)) {
    if (onCooldown("kick", 300)) return;
    const strengthStr = message.match(/!kick (\d+)/)![1];
    const strength = Math.max(1, Math.min(3, parseInt(strengthStr)));
    sail.knockbackPlayer(strength);
  }

  if (message.match(/!rave/)) {
    if (onCooldown("rave", 300)) return;
    sail.console("set gCosmetics.Link_KokiriTunic.Changed 1");
    sail.console("set gCosmetics.Link_KokiriTunic.Rainbow 1");

    setTimeout(() => {
      sail.console("set gCosmetics.Link_KokiriTunic.Changed 0");
      sail.console("set gCosmetics.Link_KokiriTunic.Rainbow 0");
    }, 1000 * 20);
  }

  if (message.match(/!tiny/)) {
    if (onCooldown("tiny", 300)) return;
    sail.modifyLinkSize(2, 10);
  }
});

sail.on("redeem", (reward, message, user) => {
  if (reward === "878f54ca-b3ec-4acd-acc1-c5482b5c2f8e") {
    sail.console("reset");
  }
});

sail.on("bits", (bits, message, user) => {
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

sail.lift("proxysaw")
  .catch((error) => {
    console.error(error);
    Deno.exit(1);
  });
