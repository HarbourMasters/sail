import { writeAll } from "https://deno.land/std@0.192.0/streams/write_all.ts";
import EventEmitter from "https://deno.land/x/eventemitter@1.2.4/mod.ts";
import { nanoid } from "https://deno.land/x/nanoid@v3.0.0/nanoid.ts";
import { Sail } from "./Sail.ts";
import {
  Hook,
  IncomingPacket,
  OnActorInitHook,
  OnEnemyDefeatHook,
  OnExitGameHook,
  OnFlagSetHook,
  OnFlagUnsetHook,
  OnItemReceiveHook,
  OnLoadGameHook,
  OnSceneFlagSetHook,
  OnSceneFlagUnsetHook,
  OnTransitionEndHook,
  OutgoingPacket,
  ResultStatus,
} from "./types.ts";

const decoder = new TextDecoder();
const encoder = new TextEncoder();

const hookToEventMap = {
  OnTransitionEnd: "transitionEnd",
  OnLoadGame: "loadGame",
  OnExitGame: "exitGame",
  OnItemReceive: "itemReceive",
  OnEnemyDefeat: "enemyDefeat",
  OnActorInit: "actorInit",
  OnFlagSet: "flagSet",
  OnFlagUnset: "flagUnset",
  OnSceneFlagSet: "sceneFlagSet",
  OnSceneFlagUnset: "sceneFlagUnset",
} as const;

export class SohClient extends EventEmitter<{
  transitionEnd: (event: OnTransitionEndHook) => void;
  loadGame: (event: OnLoadGameHook) => void;
  exitGame: (event: OnExitGameHook) => void;
  itemReceive: (event: OnItemReceiveHook) => void;
  enemyDefeat: (event: OnEnemyDefeatHook) => void;
  actorInit: (event: OnActorInitHook) => void;
  flagSet: (event: OnFlagSetHook) => void;
  flagUnset: (event: OnFlagUnsetHook) => void;
  sceneFlagSet: (event: OnSceneFlagSetHook) => void;
  sceneFlagUnset: (event: OnSceneFlagUnsetHook) => void;
  anyHook: (event: Hook) => void;
  disconnected: () => void;
}> {
  public id: number;
  private connection: Deno.Conn;
  public sail: Sail;
  private packetResolvers: {
    [id: string]: (value: ResultStatus | PromiseLike<ResultStatus>) => void;
  } = {};
  public debug = false;

  constructor(
    connection: Deno.Conn,
    sail: Sail,
    { debug }: { debug?: boolean } = {},
  ) {
    super();
    this.connection = connection;
    this.sail = sail;
    this.id = connection.rid;

    if (debug) {
      this.debug = debug;
    }

    this.log("Connected");
    this.waitForData();
  }

  async waitForData() {
    const buffer = new Uint8Array(1024);
    let data = new Uint8Array(0);

    while (true) {
      let count: null | number = 0;

      try {
        count = await this.connection.read(buffer);
      } catch (error) {
        this.log(`Error reading from connection: ${error.message}`);
        this.disconnect();
        break;
      }

      if (!count) {
        this.disconnect();
        break;
      }

      // Concatenate received data with the existing data
      const receivedData = buffer.subarray(0, count);
      data = concatUint8Arrays(data, receivedData);

      // Handle all complete packets (while loop in case multiple packets were received at once)
      while (true) {
        const delimiterIndex = findDelimiterIndex(data);
        if (delimiterIndex === -1) {
          break; // Incomplete packet, wait for more data
        }

        // Extract the packet
        const packet = data.subarray(0, delimiterIndex);
        data = data.subarray(delimiterIndex + 1);

        this.handlePacket(packet);
      }
    }
  }

  handlePacket(packet: Uint8Array) {
    try {
      const packetString = decoder.decode(packet);
      const packetObject: IncomingPacket = JSON.parse(packetString);

      if (this.debug) this.log("->", packetObject);

      if (packetObject.type === "result") {
        const resolver = this.packetResolvers[packetObject.id];
        if (resolver) {
          resolver(packetObject.status);
          delete this.packetResolvers[packetObject.id];
        }
      } else if (packetObject.type == "hook") {
        this.emit("anyHook", packetObject.hook);
        if (packetObject.hook.type in hookToEventMap) {
          this.emit(
            hookToEventMap[packetObject.hook.type],
            packetObject.hook as any,
          );
        }
      }
    } catch (error) {
      this.log(`Error handling packet: ${error.message}`);
    }
  }

  async sendPacket(packetObject: OutgoingPacket): Promise<ResultStatus> {
    try {
      if (this.debug) this.log("<-", packetObject);
      const packetString = JSON.stringify(packetObject);
      const packet = encoder.encode(packetString + "\0");

      await writeAll(this.connection, packet);

      const result: ResultStatus = await new Promise((resolve) => {
        this.packetResolvers[packetObject.id] = resolve;

        setTimeout(() => {
          if (this.packetResolvers[packetObject.id]) {
            resolve("timeout");
            delete this.packetResolvers[packetObject.id];
          }
        }, 5000);
      });

      if (result === "try_again") {
        await new Promise((resolve) => setTimeout(resolve, 500));

        return this.sendPacket(packetObject);
      }

      return result;
    } catch (error) {
      this.log(`Error sending packet: ${error.message}`);
      this.disconnect();
      return "failure";
    }
  }

  disconnect() {
    try {
      this.sail.removeClient(this);
      this.connection.close();
    } catch (error) {
      this.log(`Error disconnecting: ${error.message}`);
    } finally {
      this.emit("disconnected");
      this.log("Disconnected");
    }
  }

  // deno-lint-ignore no-explicit-any
  log(...data: any[]) {
    console.log(`[SohClient ${this.id}]:`, ...data);
  }

  /* Effect helpers */

  command(command: string) {
    return this.sendPacket({
      id: nanoid(),
      type: "command",
      command: command,
    });
  }

  knockbackPlayer(strength: number) {
    return this.sendPacket({
      id: nanoid(),
      type: "effect",
      effect: {
        type: "apply",
        name: "KnockbackPlayer",
        parameters: [strength],
      },
    });
  }

  async modifyLinkSize(size: number, lengthSeconds: number) {
    await this.sendPacket({
      id: nanoid(),
      type: "effect",
      effect: {
        type: "apply",
        name: "ModifyLinkSize",
        parameters: [size],
      },
    });

    return new Promise((resolve) => {
      setTimeout(() => {
        this.sendPacket({
          id: nanoid(),
          type: "effect",
          effect: {
            type: "remove",
            name: "ModifyLinkSize",
          },
        }).then(resolve);
      }, 1000 * lengthSeconds);
    });
  }
}

function concatUint8Arrays(a: Uint8Array, b: Uint8Array): Uint8Array {
  const result = new Uint8Array(a.length + b.length);
  result.set(a, 0);
  result.set(b, a.length);
  return result;
}

function findDelimiterIndex(data: Uint8Array): number {
  for (let i = 0; i < data.length; i++) {
    if (data[i] === 0 /* null terminator */) {
      return i;
    }
  }
  return -1;
}
