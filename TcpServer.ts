import "https://deno.land/std@0.208.0/dotenv/load.ts";
import { TcpClient } from "./TcpClient.ts";
import { Packet } from "./types.ts";

export class TcpServer {
  private listener?: Deno.Listener;
  public clients: TcpClient[] = [];

  async start() {
    try {
      const port = Deno.env.has("PORT")
        ? parseInt(Deno.env.get("PORT")!, 10)
        : 43384;
      if (isNaN(port)) {
        throw new Error("Invalid PORT environment variable");
      }

      this.listener = Deno.listen({ port: port });

      this.log(`Server listening on port ${port}`);
      for await (const connection of this.listener) {
        try {
          const client = new TcpClient(connection, this);
          this.clients.push(client);
        } catch (error) {
          this.log("Error connecting client:", error);
        }
      }
    } catch (error) {
      this.log("Error starting server:", error);
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
