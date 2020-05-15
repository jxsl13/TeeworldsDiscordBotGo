package main

import (
	"time"

	"github.com/bwmarrin/discordgo"
)

// Config contains the current bot configuration and structs
type Config struct {
	Admin           string
	FilePath        string
	DiscordSession  *discordgo.Session
	ResponseTimeout time.Duration
	ServerList      *ConcurrentServerList
}

// NewBotConfig creates a new discord configuration to work as a discord bot.
func NewBotConfig(discordToken string) (*Config, error) {
	discordSession, err := discordgo.New("Bot " + discordToken)
	if err != nil {
		return nil, err
	}

	return &Config{
		DiscordSession: discordSession}, nil
}

// Open starts the connection to the discord servers
func (c *Config) Open() error {
	return c.DiscordSession.Open()
}

// Close closes the session connection.
func (c *Config) Close() {
	c.DiscordSession.Close()
}
