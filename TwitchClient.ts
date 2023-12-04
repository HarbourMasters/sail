import * as TwitchIrc from "https://deno.land/x/twitch_irc@0.10.2/mod.ts";
import { Privmsg } from "https://deno.land/x/twitch_irc@0.10.2/lib/message/privmsg.ts";
import EventEmitter from "https://deno.land/x/eventemitter@1.2.4/mod.ts";

export class TwitchClient extends EventEmitter<{
  chat: (message: string, user: string, raw: Privmsg) => void;
  bits: (bits: number, message: string, user: string, raw: Privmsg) => void;
  redeem: (reward: string, message: string, user: string, raw: Privmsg) => void;
  raw: (raw: Privmsg) => void;
}> {
  public ircClient = new TwitchIrc.Client();

  constructor() {
    super();
    this.ircClient.on("privmsg", (e) => this.handleMessage(e));
  }

  async connect(channel: string) {
    await new Promise<void>((resolve) => {
      this.ircClient.on("open", async () => {
        await this.ircClient.join(`#${channel}`);
        this.log(`Connected to chat for ${channel}`);
        resolve();
      });
    });
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

  // deno-lint-ignore no-explicit-any
  log(...data: any[]) {
    console.log("[TwitchClient]:", ...data);
  }
}
