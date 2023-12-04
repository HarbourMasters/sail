import * as TwitchIrc from "https://deno.land/x/twitch_irc@0.10.2/mod.ts";
import { Privmsg } from "https://deno.land/x/twitch_irc@0.10.2/lib/message/privmsg.ts";
import EventEmitter from "https://deno.land/x/eventemitter@1.2.4/mod.ts";
import { TcpServer } from "./TcpServer.ts";
import { Packet } from "./types.ts";
import { nanoid } from "https://deno.land/x/nanoid@v3.0.0/nanoid.ts";

export class Sail extends EventEmitter<{
  chat: (message: string, user: string, raw: Privmsg) => void;
  bits: (bits: number, message: string, user: string, raw: Privmsg) => void;
  redeem: (reward: string, message: string, user: string, raw: Privmsg) => void;
  raw: (raw: Privmsg) => void;
}> {
  public server = new TcpServer();
  public ircClient = new TwitchIrc.Client();

  async lift(channel: string) {
    this.server.start();
    await new Promise<void>((resolve) => {
      this.ircClient.on("open", async () => {
        await this.ircClient.join(`#${channel}`);
        this.log(`Connected to chat for ${channel}`);
        resolve();
      });
    });
    this.ircClient.on("privmsg", (e) => this.handleMessage(e));
    this.log(`Sail has been lifted!`);
  }

  handleMessage(event: Privmsg) {
    this.emit("raw", event);
    if (event.raw.tags?.customRewardId) {
      this.log("Redeem Used:", event.raw.tags?.customRewardId);
      this.emit(
        "redeem",
        event.raw.tags?.customRewardId,
        event.message,
        event.user.displayName!,
        event,
      );
    } else if (event.raw.tags?.bits) {
      this.emit(
        "bits",
        parseInt(event.raw.tags!.bits, 10),
        event.message,
        event.user.displayName!,
        event,
      );
    } else {
      this.emit("chat", event.message, event.user.displayName!, event);
    }
  }

  queuePackets(packets: Packet[] | Packet) {
    this.server.queuePackets(packets);
  }

  // deno-lint-ignore no-explicit-any
  log(...data: any[]) {
    console.log("[Sail]:", ...data);
  }

  /* Effect helpers */

  command(command: string) {
    this.queuePackets({
      id: nanoid(),
      type: "effect",
      effect: {
        type: "command",
        command: command,
      },
    });
  }

  knockbackPlayer(strength: number) {
    this.queuePackets({
      id: nanoid(),
      type: "effect",
      effect: {
        type: "apply",
        name: "KnockbackPlayer",
        parameters: [strength],
      },
    });
  }

  modifyLinkSize(size: number, lengthSeconds: number) {
    this.queuePackets({
      id: nanoid(),
      type: "effect",
      effect: {
        type: "apply",
        name: "ModifyLinkSize",
        parameters: [size],
      },
    });

    setTimeout(() => {
      this.queuePackets({
        id: nanoid(),
        type: "effect",
        effect: {
          type: "remove",
          name: "ModifyLinkSize",
        },
      });
    }, 1000 * lengthSeconds);
  }
}
