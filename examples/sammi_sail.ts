import { Sail } from "../Sail.ts";
import { SohClient } from "../SohClient.ts";

let port = 43384;
if (Deno.env.has("PORT")) {
  const parsedPort = parseInt(Deno.env.get("PORT")!, 10);
  if (isNaN(parsedPort)) {
    console.warn("The PORT environment variable is not a valid number");
  } else {
    port = parsedPort;
  }
}

let httpPort = 43383;
if (Deno.env.has("HTTP_PORT")) {
  const parsedPort = parseInt(Deno.env.get("HTTP_PORT")!, 10);
  if (isNaN(parsedPort)) {
    console.warn("The HTTP_PORT environment variable is not a valid number");
  } else {
    httpPort = parsedPort;
  }
}

const sammiUrl = Deno.env.has("SAMMI_WEBHOOK_URL")
  ? Deno.env.get("SAMMI_WEBHOOK_URL")!
  : "http://localhost:9450/webhook";

const sail = new Sail({ port, debug: true });
let sohClient: SohClient | undefined;

sail.on("clientConnected", (client) => {
  sohClient = client;

  client.on("disconnected", () => {
    sohClient = undefined;
  });

  client.on("anyHook", async (event) => {
    const { type, ...rest } = event;

    try {
      await fetch(sammiUrl, {
        method: "POST",
        headers: {
          "content-type": "application/json; charset=utf-8",
        },
        body: JSON.stringify({
          trigger: type,
          ...rest,
        }),
      });
    } catch (error) {
      console.error("Error sending webhook to SAMMI", error);
    }
  });
});

async function handler(request: Request): Promise<Response> {
  if (!request.body) {
    return new Response(JSON.stringify({ status: "BAD_REQUEST" }), {
      status: 400,
      headers: {
        "content-type": "application/json; charset=utf-8",
      },
    });
  }

  const body = await request.json();
  const status = await sohClient?.sendPacket(body);

  return new Response(
    JSON.stringify({
      type: "response",
      status,
    }),
    {
      status: 200,
      headers: {
        "content-type": "application/json; charset=utf-8",
      },
    },
  );
}

(async () => {
  try {
    Deno.serve({ port: httpPort }, handler);
    await sail.start();
  } catch (error) {
    console.error("There was an error starting the Custom Sail", error);
    Deno.exit(1);
  }
})();
