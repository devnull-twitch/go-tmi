package tmi

import (
	"crypto/tls"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
	"gopkg.in/irc.v3"
)

type Client struct {
	ircClient *irc.Client
	userList  map[string]int64
	commands  map[string]Command
	modules   []Module
}

type IncomingCommand struct {
	IncomingMessage
	Command string
	Params  []string
}

type OutgoingMessage struct {
	Message     string
	Channel     string
	ParentID    string
	SendAsReply bool
}

type IncomingMessage struct {
	Message     string
	Channel     string
	MsgID       string
	Broadcaster bool
	Mod         bool
}

func New(username, token, channel, commandMarkerChar string) (*Client, error) {
	conn, err := tls.Dial("tcp", "irc.chat.twitch.tv:6697", &tls.Config{})
	if err != nil {
		return nil, err
	}

	tmiClient := &Client{
		userList: make(map[string]int64),
		commands: make(map[string]Command),
	}

	ircConfig := irc.ClientConfig{
		Nick: username,
		Pass: token,
		User: username,
		Name: username,
		Handler: irc.HandlerFunc(func(c *irc.Client, m *irc.Message) {
			if m.Command == "JOIN" && m.Prefix != nil && m.Prefix.User != "" {
				tmiClient.handleJoinFrom(m.Prefix.User)
				return
			}
			if m.Command == "PART" && m.Prefix != nil && m.Prefix.User != "" {
				tmiClient.handleUserPart(m.Prefix.User)
				return
			}
			if m.Command == "375" || m.Command == "376" || m.Command == "372" {
				// on MODT end we make join request
				if m.Command == "376" {
					c.Write(fmt.Sprintf("JOIN #%s", channel))
				}
				// MODT commands
				return
			}
			if m.Command == "001" || m.Command == "002" || m.Command == "003" || m.Command == "004" {
				// Welcome spam
				return
			}

			if m.Command == "PRIVMSG" {
				msg := IncomingMessage{
					Channel: m.Param(0),
					Message: m.Param(1),
				}
				if len(m.Tags) > 0 {
					for tagName, tagValue := range m.Tags {
						if tagName == "badges" && strings.Contains(string(tagValue), "broadcaster/1") {
							msg.Broadcaster = true
						}
						if tagName == "mod" && tagValue == "1" {
							msg.Mod = true
						}
						if tagName == "id" {
							msg.MsgID = string(tagValue)
						}
					}
				}

				if m.Param(1)[0:1] == commandMarkerChar {
					incoming := &IncomingCommand{IncomingMessage: msg}

					params := strings.Split(m.Param(1)[1:], " ")
					incoming.Command = params[0]
					incoming.Params = params[1:]

					tmiClient.handleCommand(incoming)
				}

				for _, module := range tmiClient.modules {
					args := module.MessageTrigger(tmiClient, &msg)
					if args != nil {
						out := module.Handler(tmiClient, *args)
						if out != nil {
							tmiClient.Send(out)
						}
					}
				}
			}
		}),
	}

	ircClient := irc.NewClient(conn, ircConfig)
	ircClient.CapRequest("twitch.tv/membership", true)
	ircClient.CapRequest("twitch.tv/tags", true)

	tmiClient.ircClient = ircClient

	return tmiClient, nil
}

func (c *Client) Run() error {
	logrus.Info("Starting bot")
	return c.ircClient.Run()
}
