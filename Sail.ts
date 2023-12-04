import EventEmitter from "https://deno.land/x/eventemitter@1.2.4/mod.ts";
import { TcpServer } from "./TcpServer.ts";
import { OutgoingPacket } from "./types.ts";
import { nanoid } from "https://deno.land/x/nanoid@v3.0.0/nanoid.ts";

export class Sail extends EventEmitter {
  public server = new TcpServer();

  lift() {
    this.server.start();
    this.log(`Sail has been lifted!`);
  }

  queuePackets(packets: OutgoingPacket[] | OutgoingPacket) {
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
      type: "command",
      command: command,
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
