package tmi

import (
	"strings"

	"github.com/sirupsen/logrus"
	"gopkg.in/irc.v3"
)

type (
	Parameter struct {
		Name     string
		Required bool
		Default  *string
		Validate func(string) bool
	}
	CommandArgs struct {
		Parameters             map[string]string
		RestParams             []string
		Channel                string
		ParentID               string
		UserIsBroadcasterOrMod bool
	}
	CommandHandler func(client *Client, args CommandArgs) *OutgoingMessage
	Command        struct {
		Name                     string
		Description              string
		Params                   []Parameter
		Handler                  CommandHandler
		RequiresBroadcasterOrMod bool
		AllowRestParams          bool
	}
)

func (c *Client) AddCommand(cmd Command) *Client {
	c.commands[cmd.Name] = cmd
	return c
}

func (c *Client) ListCommands() []Command {
	cmds := make([]Command, 0, len(c.commands))
	for _, c := range c.commands {
		cmds = append(cmds, c)
	}

	return cmds
}

func (c *Client) handleCommand(m *IncomingCommand) {
	l := logrus.WithField("cmd", m.Command)
	cmd, exists := c.commands[strings.ToLower(m.Command)]
	if !exists {
		l.Info("unknown command")
		return
	}

	if cmd.RequiresBroadcasterOrMod && !m.Broadcaster && !m.Mod {
		l.Info("user not premitted to execute command")
		return
	}

	pmap := make(map[string]string)
	restParams := make([]string, 0)
	for index, str := range m.Params {
		if len(cmd.Params) <= index {
			if !cmd.AllowRestParams {
				l.Warn("too many params and rest not allowed")
				return
			}
			restParams = append(restParams, str)
			continue
		}
		paramConfig := cmd.Params[index]
		pmap[paramConfig.Name] = str
	}

	for _, paramConfig := range cmd.Params {
		pl := l.WithField("param", paramConfig.Name)
		_, hasParamVal := pmap[paramConfig.Name]
		if !hasParamVal && paramConfig.Required {
			pl.Warn("missing required param")
			return
		}
		if !hasParamVal && paramConfig.Default != nil {
			pmap[paramConfig.Name] = *paramConfig.Default
		}
		if paramConfig.Validate != nil && !paramConfig.Validate(pmap[paramConfig.Name]) {
			pl.Warn("invalid param")
			return
		}
	}

	out := cmd.Handler(c, CommandArgs{
		Parameters:             pmap,
		RestParams:             restParams,
		Channel:                m.Channel,
		ParentID:               m.MsgID,
		UserIsBroadcasterOrMod: m.Broadcaster || m.Mod,
	})
	if out != nil {
		out.Channel = m.Channel
		if out.SendAsReply {
			out.ParentID = m.MsgID
		}
		c.Send(out)
	}
}

func (c *Client) Send(out *OutgoingMessage) {
	reply := &irc.Message{
		Command: "PRIVMSG",
		Params: []string{
			out.Channel,
			out.Message,
		},
	}
	if out.SendAsReply && out.ParentID != "" {
		reply.Tags = make(irc.Tags)
		reply.Tags["reply-parent-msg-id"] = irc.TagValue(out.ParentID)
	}
	c.ircClient.WriteMessage(reply)
}
