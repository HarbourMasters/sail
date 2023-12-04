## What is this?

Sail is an integration between SoH and Twitch, that allows Twitch chat to
control the game via chat commands, redeemable channel points, and bits.

## How to

- Download
  [the associated SoH Build](https://github.com/HarbourMasters/Shipwright/pull/3073)
- Install [Deno](https://deno.land/manual@v1.34.3/getting_started/installation)
- Clone this repository
- Run either the custom or JSON sail, the differences are explained below

```sh
cd <path to this repo>
deno run --allow-net --allow-read twitch_json_sail.ts
```

## Sail Types

Out of the box both types are configured with the same commands/rewards,
primarily to show you an example of how to configure each type.

### Twitch JSON Sail

The JSON sail is configuration driven by a JSON file, while it is easier to
configure and get running, it is less flexible than the custom sail. I'd start
here until you need to do something that the JSON sail doesn't support.

### Twitch Custom Sail

The custom sail is a TypeScript file that is compiled and run by Deno. It is
more flexible than the JSON sail, but requires a bit more knowledge of
programming to configure to your liking. Your limit is your imagination here.
