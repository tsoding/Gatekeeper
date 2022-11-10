# Gatekeeper

The chat bot Zozin does not want you to know about.

## Quick Start

### Gatekeeper

Gatekeeper is the bot process itself.

```console
$ go build ./cmd/gatekeeper/
$ ./gatekeeper
```

### Gaslighter

Gaslighter is the bot configuration command line tool which works by connecting and modifying the bot's database.

```console
$ go build ./cmd/gaslighter/
$ ./gaslighter
```

## Environment Variables

| Name | Description|
|---|---|
| `GATEKEEPER_DISCORD_TOKEN` | Discord token [https://discord.com/developers/docs/topics/oauth2](https://discord.com/developers/docs/topics/oauth2)|
| `GATEKEEPER_PGSQL_CONNECTION` | PostgreSQL connection URL [https://www.postgresql.org/docs/current/libpq-connect.html#id-1.7.3.8.3.6](https://www.postgresql.org/docs/current/libpq-connect.html#id-1.7.3.8.3.6) |
| `GATEKEEPER_TWITCH_IRC_NICK` | Twitch Login |
| `GATEKEEPER_TWITCH_IRC_PASS` | Twitch Password [https://twitchapps.com/tmi/](https://twitchapps.com/tmi/) |
