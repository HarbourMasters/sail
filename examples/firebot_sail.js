const { EventEmitter } = require("events");
const net = require("net");
const { promisify } = require("util");
const crypto = require("crypto");

function nanoid() {
  const randomBytes = crypto.randomBytes(16);
  return randomBytes.toString("hex");
}

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
};

class Sail extends EventEmitter {
  constructor({ port, debug } = {}) {
    super();

    this.listener = undefined;
    this.clients = [];
    this.port = port || 43384;
    this.debug = debug || false;
  }

  async start() {
    try {
      this.listener = net.createServer((connection) => {
        try {
          const client = new SohClient(connection, this, { debug: this.debug });
          this.clients.push(client);
          this.emit("clientConnected", client);
        } catch (error) {
          this.log("Error connecting client:", error);
        }
      });

      this.listener.listen(this.port, () => {
        this.log(`Server listening on port ${this.port}`);
      });
    } catch (error) {
      this.log("Error starting server:", error);
    }
  }

  stop() {
    this.clients.forEach((client) => client.disconnect());
    this.listener.close();
  }

  removeClient(client) {
    const index = this.clients.indexOf(client);
    this.clients.splice(index, 1);
  }

  log(...data) {
    console.log("[Sail]:", ...data);
  }
}

class SohClient extends EventEmitter {
  constructor(connection, sail, { debug } = {}) {
    super();
    this.packetResolvers = {};
    this.connection = connection;
    this.sail = sail;
    this.id = connection.remoteAddress + ":" + connection.remotePort;

    if (debug) {
      this.debug = debug;
    }

    this.log("Connected");
    this.waitForData();
  }

  async waitForData() {
    this.connection.on("end", () => {
      this.disconnect();
    });

    this.connection.on("error", (err) => {
      this.log(`Error: ${err}`);
    });

    let data = new Buffer.alloc(0);

    this.connection.on("data", (chunk) => {
      data = Buffer.concat([data, chunk]);

      let delimiterIndex = findDelimiterIndex(data);
      while (delimiterIndex !== -1) {
        const packet = data.slice(0, delimiterIndex);
        data = data.slice(delimiterIndex + 1);

        this.handlePacket(packet);

        delimiterIndex = findDelimiterIndex(data);
      }
    });
  }

  handlePacket(packet) {
    try {
      const packetString = decoder.decode(packet);
      const packetObject = JSON.parse(packetString);

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
          this.emit(hookToEventMap[packetObject.hook.type], packetObject.hook);
        }
      }
    } catch (error) {
      this.log(`Error handling packet: ${error.message}`);
    }
  }

  async sendPacket(packetObject) {
    try {
      if (this.debug) this.log("<-", packetObject);
      const packetString = JSON.stringify(packetObject);
      const packet = encoder.encode(packetString + "\0");

      await promisify(this.connection.write).call(this.connection, packet);

      const result = await new Promise((resolve) => {
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
      this.connection.destroy();
    } catch (error) {
      this.log(`Error disconnecting: ${error.message}`);
    } finally {
      this.emit("disconnected");
      this.log("Disconnected");
    }
  }

  log(...data) {
    console.log(`[SohClient ${this.id}]:`, ...data);
  }
}

function findDelimiterIndex(data) {
  for (let i = 0; i < data.length; i++) {
    if (data[i] === 0 /* null terminator */) {
      return i;
    }
  }
  return -1;
}

const sail = new Sail({ port: 43384, debug: true });
let sohClient;

module.exports.getScriptManifest = () => ({
  name: "Sail",
  description: "Enables remote control your Ship of Harkinian client",
  startupOnly: true,
  author: "ProxySaw",
  website: "https://github.com/harbourmasters/sail",
  version: 1,
  firebotVersion: "5",
});

module.exports.run = async (plugin) => {
  try {
    const {
      request, // http request client
      httpServer,
      effectManager, // used to register effects
      eventManager, // used to register & emit events
      eventFilterManager, // used to register event filters
      integrationManager, // used to register an integraton
      replaceVariableManager, // used to evaluate plain text via firebot's expression parser
    } = plugin.modules;
    console.log("[Sail Startup Script]: Starting");

    effectManager.registerEffect({
      definition: {
        id: "sail:knockback",
        name: "Knock back the player",
        description: "Knock back the player",
        icon: "fab fa-shoe",
        categories: ["integrations"],
        dependencies: [],
        outputs: [
          {
            label: "Result",
            description: "Returns the result of the operation",
            defaultName: "result",
          },
        ],
      },
      globalSettings: {},
      onTriggerEvent: async (event) => {
        const { effect } = event;

        try {
          if (!sohClient) {
            throw new Error("No client connected");
          }

          const result = await sohClient.sendPacket({
            id: nanoid(),
            type: "effect",
            effect: {
              type: "apply",
              name: "KnockbackPlayer",
              parameters: [3],
            },
          });

          return {
            success: true,
            outputs: {
              result,
            },
          };
        } catch (err) {
          console.error(err);
          return {
            success: false,
            outputs: {
              result,
            },
          };
        }
      },
    });

    eventManager.registerEventSource({
      id: "sail",
      name: "Sail",
      description: "Events from the Ship of Harkinian client",
      events: [
        {
          id: "transitionEnd",
          name: "Transition End",
          description: "When a scene transition ends",
          cached: false,
        },
        {
          id: "loadGame",
          name: "Load Game",
          description: "When a game is loaded",
          cached: false,
        },
        {
          id: "exitGame",
          name: "Exit Game",
          description: "When a game is exited",
          cached: false,
        },
        {
          id: "itemReceive",
          name: "Item Receive",
          description: "When an item is received",
          cached: false,
        },
        {
          id: "enemyDefeat",
          name: "Enemy Defeat",
          description: "When an enemy is defeated",
          cached: false,
        },
        {
          id: "actorInit",
          name: "Actor Init",
          description: "When an actor is initialized",
          cached: false,
        },
        {
          id: "flagSet",
          name: "Flag Set",
          description: "When a flag is set",
          cached: false,
        },
        {
          id: "flagUnset",
          name: "Flag Unset",
          description: "When a flag is unset",
          cached: false,
        },
        {
          id: "sceneFlagSet",
          name: "Scene Flag Set",
          description: "When a scene flag is set",
          cached: false,
        },
        {
          id: "sceneFlagUnset",
          name: "Scene Flag Unset",
          description: "When a scene flag is unset",
          cached: false,
        },
      ],
    });

    eventFilterManager.registerFilter({
      id: "sail:sceneNum",
      name: "Scene Num",
      description: "Filter by scene number",
      events: [{ eventSourceId: "sail", eventId: "transitionEnd" }],
      comparisonTypes: ["is", "is not"],
      valueType: "number",
      predicate: (filterSettings, eventData) => {
        const { comparisonType, value } = filterSettings;
        const { eventMeta } = eventData;

        const sceneNum = eventMeta.sceneNum;

        switch (comparisonType) {
          case "is": {
            return sceneNum === value;
          }
          case "is not": {
            return sceneNum !== value;
          }
          default:
            return false;
        }
      },
    });

    eventFilterManager.registerFilter({
      id: "sail:actorId",
      name: "Actor Id",
      description: "Filter by actor id",
      events: [
        { eventSourceId: "sail", eventId: "actorInit" },
        { eventSourceId: "sail", eventId: "enemyDefeat" },
      ],
      comparisonTypes: ["is", "is not"],
      valueType: "number",
      predicate: (filterSettings, eventData) => {
        const { comparisonType, value } = filterSettings;
        const { eventMeta } = eventData;

        const params = eventMeta.params;

        switch (comparisonType) {
          case "is": {
            return params === value;
          }
          case "is not": {
            return params !== value;
          }
          default:
            return false;
        }
      },
    });

    eventFilterManager.registerFilter({
      id: "sail:actorParams",
      name: "Actor Params",
      description: "Filter by actor params",
      events: [
        { eventSourceId: "sail", eventId: "actorInit" },
        { eventSourceId: "sail", eventId: "enemyDefeat" },
      ],
      comparisonTypes: ["is", "is not"],
      valueType: "number",
      predicate: (filterSettings, eventData) => {
        const { comparisonType, value } = filterSettings;
        const { eventMeta } = eventData;

        const actorId = eventMeta.actorId;

        switch (comparisonType) {
          case "is": {
            return actorId === value;
          }
          case "is not": {
            return actorId !== value;
          }
          default:
            return false;
        }
      },
    });

    sail.on("clientConnected", (client) => {
      sohClient = client;

      Object.keys(hookToEventMap).forEach((hookType) => {
        client.on(hookType, (hook) => {
          eventManager.triggerEvent("sail", hookType, hook);
        });
      });

      client.on("disconnected", () => {
        sohClient = undefined;
      });
    });

    await sail.start();

    return {
      success: true,
    };
  } catch (error) {
    console.error("[Sail Startup Script]:", error);
    return {
      success: false,
      errorMessage: error.message,
    };
  }
};

module.exports.stop = () => {
  console.log("[Sail Startup Script]: Stopping");

  sail.stop();
};
