import { Sail } from "../Sail.ts";
import { SohClient } from "../SohClient.ts";
import { TwitchClient } from "../TwitchClient.ts";
import { nanoid } from "https://deno.land/x/nanoid@v3.0.0/nanoid.ts";

const sail = new Sail({ port: 43384, debug: true });
const twitchClient = new TwitchClient({ channel: "caladius" });
let sohClient: SohClient | undefined;

twitchClient.on("chat", (message, user) => {
  if (message.match(/\d/)) {
    sohClient?.sendPacket({
      id: nanoid(),
      type: "effect",
      effect: {
        type: "apply",
        name: "ChaosVote",
        parameters: [parseInt(message, 10), user],
      },
    });
  }
});

sail.on("clientConnected", (client) => {
  sohClient = client;

  client.on("disconnected", () => {
    sohClient = undefined;
  });
});

(async () => {
  try {
    await twitchClient.connect();
    await sail.start();
  } catch (error) {
    console.error("There was an error starting the Custom Sail", error);
    Deno.exit(1);
  }
})();
