package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
	"github.com/jxsl13/twapi/browser"
)

var (
	errCacheEmpty = "There are currently no servers in the cache, please wait a moment and try again."
)

func main() {

	env, err := godotenv.Read(".env")
	if err != nil {
		log.Fatal(err)
	}

	discordToken := env["DISCORD_TOKEN"]

	if discordToken == "" {
		log.Fatal("error: no DISCORD_TOKEN specified")
	}

	cm := browser.NewConcurrentMap(512)

	dg, err := discordgo.New("Bot " + discordToken)
	if err != nil {
		log.Fatal(err)
	}

	dg.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {

		// Ignore all messages created by the bot itself
		// This isn't required in this specific example but it's a good practice.
		if m.Author.ID == s.State.User.ID {
			return
		}

		if !strings.HasPrefix(m.Content, "!") {
			return
		}

		ss := strings.SplitN(m.Content[1:], " ", 2)
		if len(ss) == 0 {
			return
		}

		command := ss[0]
		arguments := ""

		if len(ss) > 1 {
			arguments = ss[1]
		}

		switch command {
		case "help":
			s.ChannelMessageSend(m.ChannelID, `
Teeworlds Discord Bot by jxsl13. Have fun.
Commands:
**!p[layer]** <player> -  Check whether a player is currently online
**!o[nline]** <gametype> - Find all online servers with a specific gametype
**!o[nline]p[layers]** <gametype> - Show a list of servers and players playing a specific gametype.
**!s[ervers]** - Shows a number of registered servers.
`)
		case "s", "servers":
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("There are currently %d servers online.", cm.Len()))
		case "p", "player":

			playername := arguments

			servers := cm.Values()

			if len(servers) == 0 {
				_, err := s.ChannelMessageSend(m.ChannelID, "No servers found!")
				if err != nil {
					log.Println("Failed to send answer.")
				}
				return
			}
			sort.Sort(byPlayerCountDescending(servers))

			var sb strings.Builder

			foundPlayer := false
			for _, server := range servers {
				found := false
				for _, player := range server.Players {
					if strings.Contains(strings.ToLower(player.Name), strings.ToLower(playername)) {
						found = true
						break
					}
				}

				if !found {
					continue
				}
				foundPlayer = true

				fmt.Fprintf(&sb, "**%s** (%d Players):\n", server.Name, len(server.Players))
				fmt.Fprintf(&sb, "```\n")
				for _, player := range server.Players {
					if strings.Contains(strings.ToLower(player.Name), strings.ToLower(playername)) {
						fmt.Fprintf(&sb, "%-20s  %-16s\n", player.Name, player.Clan)
					}
				}
				fmt.Fprintf(&sb, "```\n")
			}

			if !foundPlayer {
				s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Did not find '%s'\n", playername))
				return
			}
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Found '%s' on:\n%s", playername, sb.String()))

		case "o", "online":
			gametype := arguments

			servers := cm.Values()

			if len(servers) == 0 {
				_, err := s.ChannelMessageSend(m.ChannelID, "There are currently no servers in the cache.")
				if err != nil {
					log.Println("Failed to send answer.")
				}
				return
			}
			sort.Sort(byPlayerCountDescending(servers))

			resultServers := make([]*browser.ServerInfo, 0, 10)
			for _, server := range servers {
				server := server
				if len(server.Players) == 0 {
					break
				}

				if !strings.Contains(strings.ToLower(server.GameType), strings.ToLower(gametype)) {
					continue
				}

				resultServers = append(resultServers, &server)
			}

			if len(resultServers) == 0 {
				s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("No servers found."))
				return
			}

			var sb strings.Builder

			for _, server := range resultServers {
				fmt.Fprintf(&sb, "**%s** (%d Players)\n", server.Name, len(server.Players))
			}

			for _, line := range strings.Split(sb.String(), "\n") {
				s.ChannelMessageSend(m.ChannelID, line)
			}

		case "op", "onlineplayers":
			gametype := arguments

			servers := cm.Values()

			if len(servers) == 0 {
				_, err := s.ChannelMessageSend(m.ChannelID, "There are currently no servers in the cache.")
				if err != nil {
					log.Println("Failed to send answer.")
				}
				return
			}
			sort.Sort(byPlayerCountDescending(servers))

			resultServers := make([]*browser.ServerInfo, 0, 10)
			for _, server := range servers {
				server := server
				if len(server.Players) == 0 {
					break
				}

				if !strings.Contains(strings.ToLower(server.GameType), strings.ToLower(gametype)) {
					continue
				}

				resultServers = append(resultServers, &server)
			}

			if len(resultServers) == 0 {
				s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("No servers found."))
				return
			}

			var sb strings.Builder

			for _, server := range resultServers {
				fmt.Fprintf(&sb, "**%s** (%d Players)\n", server.Name, len(server.Players))
				fmt.Fprint(&sb, "```\n")

				for _, player := range server.Players {
					fmt.Fprintf(&sb, "%-20s  %-16s\n", player.Name, player.Clan)
				}
				fmt.Fprint(&sb, "```\n")

				s.ChannelMessageSend(m.ChannelID, sb.String())
				sb.Reset()
			}
		default:
			return
		}
	})

	err = dg.Open()
	if err != nil {
		log.Fatalf("error: could not establish a connection to the discord api, please check your credentials")
	}
	defer dg.Close()

	go func() {
		num := 0
		for {
			num = cm.Cleanup()
			log.Printf("cleaned up %d servers", num)
			time.Sleep(browser.TokenExpirationDuration)
		}
	}()

	go func() {
		for {
			infos := browser.ServerInfos()
			log.Printf("updated %d server.", len(infos))
			for _, info := range infos {
				cm.Add(info, 20*time.Second)
			}

			// set bot status to the number of online servers.
			dg.UpdateListeningStatus(fmt.Sprintf("%d servers", len(infos)))

			log.Printf("%d servers in cache", cm.Len())
		}
	}()

	// Wait here until CTRL-C or other term signal is received.
	log.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc
	log.Println("Shutting down, please wait...")
}

type byPlayerCountDescending []browser.ServerInfo

func (a byPlayerCountDescending) Len() int           { return len(a) }
func (a byPlayerCountDescending) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byPlayerCountDescending) Less(i, j int) bool { return len(a[i].Players) > len(a[j].Players) }
