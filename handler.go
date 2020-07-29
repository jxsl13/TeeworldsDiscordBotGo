package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"sort"
	"strconv"
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

	if config.DefaultGameTypeFilter != "" {
		filter := strings.ToLower(config.DefaultGameTypeFilter)
		formated := fmt.Sprintf("	**!online [%s]**  - List all registered servers that have players playing(**!o [%s]**).\n", filter, filter)
		sb.WriteString(formated)
	} else {
		sb.WriteString("	**!online [gametype]**  - List all registered servers that have players playing(**!o [gametype]**).\n")
	}

	sb.WriteString("	**!servers** - Show all servers that are currently registered(**!s**).\n")
	s.ChannelMessageSend(m.ChannelID, sb.String())
}

// OnlineHandler handler the !online command
func OnlineHandler(s *discordgo.Session, m *discordgo.MessageCreate, args string) {
	gametype := strings.ToLower(strings.TrimSpace(args))

	// set default filter
	if gametype == "" && config.DefaultGameTypeFilter != "" {
		gametype = config.DefaultGameTypeFilter
	}

	infos := fetchServerInfos()

	filteredServers := make([]browser.ServerInfo, 0, len(infos))

	for _, server := range infos {

		if len(server.Players) == 0 {
			continue
		}

		if gametype == "" || (gametype != "" && strings.Contains(strings.ToLower(server.GameType), gametype)) {
			filteredServers = append(filteredServers, server)
		}
	}

	if len(filteredServers) == 0 {
		s.ChannelMessageSend(m.ChannelID, "no online servers found.")
		return
	}

	sort.Sort(byPlayerCountDescending(filteredServers))

	sb := strings.Builder{}
	sb.Grow(2000)

	for _, server := range filteredServers {

		sb.WriteString(fmt.Sprintf("**%s** - Map: **%s** (%d/%2d)\n", Escape(server.Name), Escape(server.Map), server.NumClients, server.MaxClients))

		for _, player := range server.Players {
			inlineCode := WrapInInlineCodeBlock(fmt.Sprintf("%-20s %-16s", player.Name, player.Clan))
			sb.WriteString(fmt.Sprintf("%s %s \n", Flag(player.Country), inlineCode))

			if sb.Len() > 1800 {
				s.ChannelMessageSend(m.ChannelID, sb.String())
				sb.Reset()
			}
		}

		// only send if threshold exceeded to send less messages with more text
		if sb.Len() > 1700 {
			s.ChannelMessageSend(m.ChannelID, sb.String())
			sb.Reset()
		}
	}

	// send remaining text
	if sb.Len() > 0 {
		s.ChannelMessageSend(m.ChannelID, sb.String())
	}

}

// ServersHandler handles the !servers command
func ServersHandler(s *discordgo.Session, m *discordgo.MessageCreate, args string) {
	infos := fetchServerInfos()

	sort.Sort(byPlayerCountDescending(infos))

	sb := strings.Builder{}
	sb.Grow(2000)

	fetchedServers := 0
	for _, server := range infos {
		if server.Name != "" {
			fetchedServers++
		}
	}

	if fetchedServers == 0 {
		s.ChannelMessageSend(m.ChannelID, "could not fetch any server infos.")
		return
	}

	for _, server := range infos {

		if server.Name == "" {
			sb.WriteString(fmt.Sprintf("Failed to fetch: %s\n", server.Address))
		} else {
			playersFormat := fmt.Sprintf("(%d/%d)", server.NumClients, server.MaxClients)
			lineFormat := fmt.Sprintf("**%s** Address: %s Map: **%s** %7s\n", Escape(server.Name), server.Address, Escape(server.Map), playersFormat)
			sb.WriteString(lineFormat)
		}

		if sb.Len() > 1000 {
			s.ChannelMessageSend(m.ChannelID, sb.String())
			sb.Reset()
		}
	}

	if sb.Len() > 0 {
		s.ChannelMessageSend(m.ChannelID, sb.String())
	}
}

func fetchServerInfos() []browser.ServerInfo {
	numServers := config.ServerList.Len()
	cm := browser.NewConcurrentMap(numServers)

	wg := sync.WaitGroup{}
	wg.Add(numServers)

	for _, addr := range config.ServerList.List() {
		go fetchServerInfoFromServerAddress(addr, config.ResponseTimeout, &cm, &wg)
	}

	wg.Wait()

	return cm.Values()
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
		// no server name -> failed to fetch
		cm.Add(browser.ServerInfo{Address: srv.String(), Name: ""}, 0)
		return
	}

	info, err := browser.ParseServerInfo(resp, srv.String())
	if err != nil {
		// no server name -> failed to fetch
		cm.Add(browser.ServerInfo{Address: srv.String(), Name: ""}, 0)
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

	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_TRUNC|os.O_CREATE, 0600)
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

// ClearHandler handles the !clear command that removes no accessible servers.
func ClearHandler(s *discordgo.Session, m *discordgo.MessageCreate, args string) {

	infos := fetchServerInfos()

	serverMap := make(map[string]int, len(infos))

	for _, fetchedInfo := range infos {
		if fetchedInfo.Name != "" {
			serverMap[fetchedInfo.Address]++
		}
	}

	retries := 3

	if r, err := strconv.Atoi(strings.TrimSpace(args)); err != nil && r > 1 {
		retries = r
	}

	for i := 0; i < retries; i++ {
		infos := fetchServerInfos()

		for _, fetchedInfo := range infos {
			if fetchedInfo.Name != "" {
				serverMap[fetchedInfo.Address]++
			}
		}
	}

	knownServers := config.ServerList.SortedList()

	sb := strings.Builder{}

	for _, knownServer := range knownServers {
		address := knownServer.String()
		if serverMap[address] == 0 {
			config.ServerList.Delete(address)
			sb.WriteString(fmt.Sprintf("removed: %s\n", address))

			if sb.Len() > 1800 {
				s.ChannelMessageSend(m.ChannelID, sb.String())
				sb.Reset()
			}
		}
	}

	if sb.Len() > 0 {
		s.ChannelMessageSend(m.ChannelID, sb.String())
		sb.Reset()
	}
}

// AdminMessageCreateMiddleware is a wrapper that wraps around specific handler functions in order to deny access to non-admin users.
func AdminMessageCreateMiddleware(next MessageCreateHandler) MessageCreateHandler {
	return func(s *discordgo.Session, m *discordgo.MessageCreate, args string) {
		if config.Admin == "" || m.Author.String() != config.Admin {
			s.ChannelMessageSend(m.ChannelID, "you are not allowed to access this command.")
			return
		}
		next(s, m, args)
	}
}
