import { TcpClient } from "./TcpClient.ts";
import { Packet } from "./types.ts";

export class TcpServer {
  private listener?: Deno.Listener;
  public clients: TcpClient[] = [];

  async start() {
    this.listener = Deno.listen({ port: 43384 });

    this.log("Server listening on port 43384");
    try {
      for await (const connection of this.listener) {
        try {
          const client = new TcpClient(connection, this);
          this.clients.push(client);
        } catch (error) {
          this.log(`Error connecting client: ${error.message}`);
        }
      }
    } catch (error) {
      this.log(`Error starting server: ${error.message}`);
    }
  }

  removeClient(client: TcpClient) {
    const index = this.clients.indexOf(client);
    this.clients.splice(index, 1);
  }

  queuePackets(packets: Packet[] | Packet) {
    this.clients.forEach((client) => client.queuePackets(packets));
  }

  // deno-lint-ignore no-explicit-any
  log(...data: any[]) {
    console.log("[TcpServer]:", ...data);
  }
}
