# TeeworldsDiscordBotGo

A discord bot that caches current teeworlds server data in order to query for online players and stuff.

Install dependencies & build

```bash
go get -d
go build
```

Create `.env` file

```bash
touch .env
```

Add your discord application token to the `.env` file

```ini
DISCORD_TOKEN=<SECRET TOKEN>
DISCORD_ADMIN="jxsl13#5272"
DEFAULT_GAMETYPE_FILTER=zCatch

# how long to wait before saying that the server is not reachable
SERVER_RESPONSE_TIMEOUT_MS=500
```

Run the bot (the text file must exist, but can be empty)

```bash
./TeeworldsDiscordBotGo -f text_file_with_ips.txt
```

Show available commands

```discord
!help
```
