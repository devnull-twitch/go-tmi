package tmi

import (
	"crypto/tls"
	"strings"

	"github.com/sirupsen/logrus"
	"gopkg.in/irc.v3"
)

type (
	StartCallback func()

	Client struct {
		ircClient       *irc.Client
		userList        map[string]int64
		commands        map[string]Command
		modules         []Module
		startupFunction StartCallback
	}

	IncomingCommand struct {
		IncomingMessage
		Command string
		Params  []string
	}

	OutgoingMessage struct {
		Message     string
		Channel     string
		ParentID    string
		SendAsReply bool
	}

	IncomingMessage struct {
		Message       string
		Username      string
		Channel       string
		MsgID         string
		ParentID      string
		ReplyUsername string
		Broadcaster   bool
		Mod           bool
		Subscriber    bool
		VIP           bool
	}
)

func New(username, token, commandMarkerChar string) (*Client, error) {
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
				// MODT commands
				if m.Command == "376" && tmiClient.startupFunction != nil {
					tmiClient.startupFunction()
				}
				return
			}
			if m.Command == "001" || m.Command == "002" || m.Command == "003" || m.Command == "004" {
				// Welcome spam
				return
			}

			if m.Command == "PRIVMSG" {
				msgStr := m.Param(1)
				var replyUsername string
				if len(msgStr) > 1 && msgStr[0:1] == "@" && strings.Contains(msgStr, " ") {
					si := strings.Index(msgStr, " ")
					replyUsername = msgStr[1:si]
					if len(msgStr) > si+1 {
						msgStr = msgStr[si+1:]
					}
				}
				msg := IncomingMessage{
					Channel:       m.Param(0)[1:],
					Message:       msgStr,
					Username:      m.User,
					ReplyUsername: replyUsername,
				}
				if len(m.Tags) > 0 {
					for tagName, tagValue := range m.Tags {
						if tagName == "badges" && strings.Contains(string(tagValue), "broadcaster/1") {
							msg.Broadcaster = true
						}
						if tagName == "mod" && tagValue == "1" {
							msg.Mod = true
						}
						if tagName == "subscriber" && tagValue == "1" {
							msg.Subscriber = true
						}
						if tagName == "badges" && strings.Contains(string(tagValue), "vip") {
							msg.Subscriber = true
						}
						if tagName == "badges" && strings.Contains(string(tagValue), "founder") {
							msg.VIP = true
						}
						if tagName == "id" {
							msg.MsgID = string(tagValue)
						}
						if tagName == "reply-parent-msg-id" {
							msg.ParentID = string(tagValue)
						}
					}
				}

				if msgStr[0:1] == commandMarkerChar {
					incoming := &IncomingCommand{IncomingMessage: msg}

					params := make([]string, 0)
					strReader := strings.NewReader(msgStr[1:])

					currentParam := &strings.Builder{}
					inQuotes := false
					for strReader.Len() > 0 {
						char, _, err := strReader.ReadRune()
						if err != nil {
							logrus.WithError(err).Error("error reading chat message")
							return
						}

						switch char {
						case ' ':
							if inQuotes {
								currentParam.WriteRune(char)
							} else {
								if currentParam.Len() <= 0 {
									continue
								} else {
									params = append(params, currentParam.String())
									currentParam.Reset()
								}
							}
						case '"':
							inQuotes = !inQuotes
						default:
							currentParam.WriteRune(char)
						}
					}

					if currentParam.Len() > 0 {
						params = append(params, currentParam.String())
					}

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

func (c *Client) AfterStartup(cb StartCallback) {
	c.startupFunction = cb
}

func (c *Client) Run() error {
	logrus.Info("Starting bot")
	return c.ircClient.Run()
}
