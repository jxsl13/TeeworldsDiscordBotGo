package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/jxsl13/twapi/browser"
)

// MessageCreateHandler is a function that handles a newly created user message
type MessageCreateHandler func(*discordgo.Session, *discordgo.MessageCreate, string)

// MessageCreateMiddleware is a wrapper fucntion
type MessageCreateMiddleware func(MessageCreateHandler) MessageCreateHandler

// HelpHandler shows the help message
func HelpHandler(s *discordgo.Session, m *discordgo.MessageCreate, args string) {

	sb := strings.Builder{}
	sb.WriteString("Teeworlds Discord Bot by jxsl13. Have fun.\n")
	sb.WriteString("Commands:\n")
	sb.WriteString("	**!online**  - List all registered servers that have players playing(**!o**).\n")
	sb.WriteString("	**!servers** - Show all servers that are currently registered(**!s**).\n")
	s.ChannelMessageSend(m.ChannelID, sb.String())
}

// OnlineHandler handler the !online command
func OnlineHandler(s *discordgo.Session, m *discordgo.MessageCreate, args string) {
	numServers := config.ServerList.Len()
	cm := browser.NewConcurrentMap(numServers)

	wg := sync.WaitGroup{}
	wg.Add(numServers)

	for _, addr := range config.ServerList.List() {
		go fetchServerInfoFromServerAddress(addr, config.ResponseTimeout, &cm, &wg)
	}

	wg.Wait()

	infos := cm.Values()

	sort.Sort(byPlayerCountDescending(infos))

	sb := strings.Builder{}
	sb.Grow(2000)

	for _, server := range infos {

		if len(server.Players) == 0 {
			continue
		}

		sb.WriteString(fmt.Sprintf("**%s** (%2d/%2d)\n", server.Name, server.NumClients, server.MaxClients))

		if len(server.Players) > 0 {

			//sb.WriteString("```")
			for _, player := range server.Players {
				sb.WriteString(fmt.Sprintf("%s `%-20s %-16s`\n", Flag(player.Country), player.Name, player.Clan))
			}
			//sb.WriteString("```")
		}

		s.ChannelMessageSend(m.ChannelID, sb.String())
		sb.Reset()
	}

}

// ServersHandler handles the !servers command
func ServersHandler(s *discordgo.Session, m *discordgo.MessageCreate, args string) {
	numServers := config.ServerList.Len()
	cm := browser.NewConcurrentMap(numServers)

	wg := sync.WaitGroup{}
	wg.Add(numServers)

	for _, addr := range config.ServerList.List() {
		go fetchServerInfoFromServerAddress(addr, config.ResponseTimeout, &cm, &wg)
	}

	wg.Wait()

	infos := cm.Values()

	sort.Sort(byPlayerCountDescending(infos))

	sb := strings.Builder{}
	sb.Grow(2000)

	for _, server := range infos {

		sb.WriteString(fmt.Sprintf("**%s** %7s\n", server.Name, fmt.Sprintf("(%d/%d)", server.NumClients, server.MaxClients)))

		if sb.Len() > 1000 {
			s.ChannelMessageSend(m.ChannelID, sb.String())
			sb.Reset()
		}
	}

	if sb.Len() > 0 {
		s.ChannelMessageSend(m.ChannelID, sb.String())
	}
}

func fetchServerInfoFromServerAddress(srv *net.UDPAddr, timeout time.Duration, cm *browser.ConcurrentMap, wg *sync.WaitGroup) {
	defer wg.Done()

	conn, err := net.DialUDP("udp", nil, srv)
	if err != nil {
		return
	}
	defer conn.Close()

	const maxBufferSize = 1500
	// increase buffers for writing and reading
	conn.SetReadBuffer(maxBufferSize)
	conn.SetWriteBuffer(int(maxBufferSize * timeout.Seconds()))

	resp, err := browser.Fetch("serverinfo", conn, timeout)
	if err != nil {
		return
	}

	info, err := browser.ParseServerInfo(resp, srv.String())
	if err != nil {
		return
	}

	cm.Add(info, 0)
}

// AddHandler handles the !add command
func AddHandler(s *discordgo.Session, m *discordgo.MessageCreate, args string) {
	err := config.ServerList.Add(args)

	if err != nil {
		s.ChannelMessageSend(m.ChannelID, err.Error())
		return
	}
	s.ChannelMessageSend(m.ChannelID, "Added.")
}

// SaveHandler handles the !add command
func SaveHandler(s *discordgo.Session, m *discordgo.MessageCreate, args string) {

	servers := config.ServerList.SortedList()
	filePath := config.FilePath

	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_TRUNC|os.O_CREATE, 0660)
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "Failed to create file.")
		return
	}
	defer file.Close()

	sb := strings.Builder{}
	sb.Grow(len(servers) * 26) // IP:port\n (max. 26 characters)

	w := bufio.NewWriter(file)

	for _, server := range servers {
		w.WriteString(fmt.Sprintf("%s\n", server.String()))
	}
	err = w.Flush()
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "Failed to write to file.")
		return
	}

	s.ChannelMessageSend(m.ChannelID, "Successfully saved to file.")
}

// DeleteHandler handles the !add command
func DeleteHandler(s *discordgo.Session, m *discordgo.MessageCreate, args string) {
	err := config.ServerList.Delete(args)

	if err != nil {
		s.ChannelMessageSend(m.ChannelID, err.Error())
		return
	}
	s.ChannelMessageSend(m.ChannelID, "Deleted.")
}

// AdminMessageCreateMiddleware is a wrapper that wraps around specific handler functions in order to deny access to non-admin users.
func AdminMessageCreateMiddleware(next MessageCreateHandler) MessageCreateHandler {
	return func(s *discordgo.Session, m *discordgo.MessageCreate, args string) {
		if len(config.Admin) == 0 || m.Author.String() != config.Admin {
			s.ChannelMessageSend(m.ChannelID, "you are not allowed to access this command.")
			return
		}
		next(s, m, args)
	}
}
