import { writeAll } from "https://deno.land/std@0.192.0/streams/write_all.ts";
import { TcpServer } from "./TcpServer.ts";
import { Packet } from "./types.ts";

const decoder = new TextDecoder();
const encoder = new TextEncoder();

export class TcpClient {
  public id: number;
  private connection: Deno.Conn;
  public server: TcpServer;
  private waitingOnResponse = false;
  private packetQueue: Packet[] = [];

  constructor(connection: Deno.Conn, server: TcpServer) {
    this.connection = connection;
    this.server = server;
    this.id = connection.rid;

    this.log("Connected");
    this.waitForData();
    this.processQueue();
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

  async processQueue() {
    if (this.packetQueue.length && !this.waitingOnResponse) {
      console.log("processing");
      const command = this.packetQueue.shift()!;
      this.packetQueue.push(command);
      this.waitingOnResponse = true;
      this.log("Sending: " + command.id);
      await this.sendPacket(command);
    }

    setTimeout(() => {
      this.processQueue();
    }, 100);
  }

  handlePacket(packet: Uint8Array) {
    try {
      const packetString = decoder.decode(packet);
      const packetObject: Packet = JSON.parse(packetString);

      this.log(`-> ${packetObject.id} packet`);

      this.packetQueue = this.packetQueue.filter(
        (payload) => payload.id !== packetObject.id,
      );
      this.waitingOnResponse = false;
    } catch (error) {
      this.log(`Error handling packet: ${error.message}`);
    }
  }

  async sendPacket(packetObject: Packet) {
    try {
      this.log(`<- ${packetObject.id} packet`);
      const packetString = JSON.stringify(packetObject);
      const packet = encoder.encode(packetString + "\0");

      await writeAll(this.connection, packet);
    } catch (error) {
      this.log(`Error sending packet: ${error.message}`);
      this.disconnect();
    }
  }

  queuePackets(packets: Packet[] | Packet) {
    if (Array.isArray(packets)) {
      this.packetQueue.push(...packets);
    } else {
      this.packetQueue.push(packets);
    }
  }

  disconnect() {
    try {
      this.server.removeClient(this);
      this.connection.close();
    } catch (error) {
      this.log(`Error disconnecting: ${error.message}`);
    } finally {
      this.log("Disconnected");
    }
  }

  // deno-lint-ignore no-explicit-any
  log(...data: any[]) {
    console.log(`[TcpClient ${this.id}]:`, ...data);
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
