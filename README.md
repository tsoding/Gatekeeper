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
| `GATEKEEPER_SOWON2_HTTP_ADDRESS` | Address for the Sowon2 HTTP control to listen to. Format is `<ip>:<port>`. |

## Sowon2 Control

When `GATEKEEPER_SOWON2_HTTP_ADDRESS` is provided the bot starts up REST API on that address and port over plain HTTP (no HTTPS). It is intended to be used in conjunction with Sowon2 to make the bot report currently playing songs. Sowon2 is a second iteration of the [Sowon](https://github.com/tsoding/sowon) timer which also integrate a music player. The source code and the binaries of Sowon2 are not publically available as of right now to avoid additional maintenance burden.

### REST API

#### `POST /song`

Makes the bot send out a notification in the Twitch chat about a currently playing song.

##### [Form Data fields](https://en.wikipedia.org/wiki/Percent-encoding#The_application.2Fx-www-form-urlencoded_type)

| Field name | Description                          |
|------------|--------------------------------------|
| `artist`   | The artist of the song (duh)         |
| `title`    | The title of the song (duh)          |
| `link`     | The link to the song on the Internet |

### Security Considerations

Only plain HTTP without any authentication is supported. Because of that it is highly advised to set the address of `GATEKEEPER_SOWON2_HTTP_ADDRESS` to `127.0.0.1:<port>` and connect to it via an [SSH tunnel](https://www.ssh.com/academy/ssh/tunneling) if the bot is hosted on a remote machine.

### Quick Start via SSH tunnel

Make sure `GATEKEEPER_SOWON2_HTTP_ADDRESS` is set to `127.0.0.1:8080` on your `remotemachine` host.

Establish the SSH tunnel:

```console
$ ssh user@remotemachine -N -L 8080:127.0.0.1:8080
```

Send out the notification:

```console
$ curl -X POST -d "artist=Rick Astley&title=Never Gonna Give You Up&link=https://www.youtube.com/watch?v=dQw4w9WgXcQ" http://127.0.0.1:8080/song
```

To keep the tunnel stable consider using [autossh](https://www.harding.motd.ca/autossh/):

```console
$ autossh -M 0 -o "ServerAliveInterval 30" -o "ServerAliveCountMax 3" -N -L 8080:127.0.0.1:8080 user@remotemachine
```
