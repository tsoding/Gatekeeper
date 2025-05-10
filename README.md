# Gatekeeper

<p align="center">
  <img src="./logo/gatekeeper-256.png" alt="logo" />
</p>

The chat bot Zozin does not want you to know about.

## Quick Start

### Gatekeeper

Gatekeeper is the bot process itself.

```console
$ go build ./cmd/gatekeeper/
$ ./gatekeeper
```

### Gaslighter

Gaslighter is the bot configuration command line tool which works by connecting to the bot's database and modifying it.

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
| `GATEKEEPER_MPV_IPC_ADDRESS` | Address for the MPV IPC control to listen to. Format is `<ip>:<port>` |

## MPV Control

When `GATEKEEPER_MPV_IPC_ADDRESS` is provided bot starts listening for [MPV IPC](https://mpv.io/manual/stable/#json-ipc) on that address and port. It is intended to be used in conjunction with [mpv-client](./cmd/mpv-client) to make the bot report currently playing songs.

### Security Considerations

The connection and the protocol are insecure and lack any authentication or encryption. Because of that it is highly advised to set the address of `GATEKEEPER_MPV_IPC_ADDRESS` to `127.0.0.1:<port>` and connect the mpv-client through an [SSH tunnel](https://www.ssh.com/academy/ssh/tunneling) if the bot is hosted on a remote machine.

### Quick Start via SSH tunnel

Make sure `GATEKEEPER_MPV_IPC_ADDRESS` is set to `127.0.0.1:8080` on your `remotemachine` host.

Build the mpv-client:

```console
$ cc -o mpv-client ./cmd/mpv-client/mpv-client.c
```

Establish the SSH tunnel:

```console
$ ssh user@remotemachine -N -L 8080:127.0.0.1:8080
```

Play the song with the mpv-client (make sure [mpv](https://mpv.io/) is installed as the mpv-client wraps around it).

```console
$ ./mpv-client song.mp3
```
