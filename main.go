package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
)

const (
	errCacheEmpty = "There are currently no servers in the cache, please wait a moment and try again."
)

var (
	config         = &Config{}
	extractIPRegex = regexp.MustCompile(`([a-fA-F:.0-9]{7,40}):(\d+)`)
)

type ipPort struct {
	IP   string
	Port string
}

func init() {
	env, err := godotenv.Read(".env")
	if err != nil {
		log.Fatal(err)
	}

	discordToken := env["DISCORD_TOKEN"]

	if discordToken == "" {
		log.Fatal("error: no DISCORD_TOKEN specified")
	}

	config, err = NewBotConfig(discordToken)

	if err != nil {
		log.Fatal(err)
	}

	config.Admin = env["DISCORD_ADMIN"]
	config.DefaultGameTypeFilter = strings.ToLower(strings.TrimSpace(env["DEFAULT_GAMETYPE_FILTER"]))

	fileName := ""

	flag.StringVar(&fileName, "f", "", "pass the file that contains the IPs that the bot is allowed to ping for infos.")
	flag.Parse()

	if fileName == "" {
		flag.Usage()
		log.Fatal("")
	}

	file, err := os.Open(fileName)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	config.FilePath = fileName

	addressSet := make(map[ipPort]bool, 1)
	sc := bufio.NewScanner(file)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())

		if strings.HasPrefix(line, "#") {
			continue
		}
		matches := extractIPRegex.FindStringSubmatch(line)
		if len(matches) != 3 {
			log.Printf("'%s' invalid line format, skipping..\n", line)
			continue
		}
		address := ipPort{matches[1], matches[2]}
		addressSet[address] = true
	}

	config.ServerList = NewConcurrentServerList(len(addressSet))
	for addr := range addressSet {

		// validate IP
		ip := net.ParseIP(addr.IP)
		if ip == nil {
			log.Printf("invalid IP '%s', with port '%s", ip, addr.Port)
			continue
		}

		// validate Port
		port, err := strconv.Atoi(addr.Port)
		if err != nil || port < 1024 {
			log.Printf("invalid port '%d', with IP '%s", port, ip)
			continue
		}

		// add server to list
		config.ServerList.Add(fmt.Sprintf("%s:%d", ip, port))
	}

	responseTimeoutMsStr := env["SERVER_RESPONSE_TIMEOUT_MS"]

	responseTimeoutMs, err := strconv.Atoi(responseTimeoutMsStr)
	if err != nil || responseTimeoutMs < 5 {
		responseTimeoutMs = 500
	}

	config.ResponseTimeout = time.Millisecond * time.Duration(responseTimeoutMs)

	config.DiscordSession.AddHandler(DiscordMessageCreateHandler)

}

// DiscordMessageCreateHandler handles server messages sent by users.
func DiscordMessageCreateHandler(s *discordgo.Session, m *discordgo.MessageCreate) {
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

	command := strings.ToLower(ss[0])
	arguments := ""

	if len(ss) > 1 {
		arguments = strings.TrimSpace(ss[1])
	}

	switch command {
	case "h", "help":
		HelpHandler(s, m, arguments)
	case "o", "online":
		OnlineHandler(s, m, arguments)
	case "s", "servers":
		ServersHandler(s, m, arguments)
	case "add":
		AdminMessageCreateMiddleware(AddHandler)(s, m, arguments)
	case "save":
		AdminMessageCreateMiddleware(SaveHandler)(s, m, arguments)
	case "delete":
		AdminMessageCreateMiddleware(DeleteHandler)(s, m, arguments)
	case "c", "clean", "clear":
		AdminMessageCreateMiddleware(ClearHandler)(s, m, arguments)
	default:
		return
	}
}

func main() {

	err := config.Open()
	if err != nil {
		log.Fatalf("error: could not establish a connection to the discord api, please check your credentials")
	}
	defer config.Close()

	// Wait here until CTRL-C or other term signal is received.
	log.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc
	log.Println("Shutting down, please wait...")
}
