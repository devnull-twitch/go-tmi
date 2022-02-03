package tmi

import (
	"time"

	"github.com/sirupsen/logrus"
)

func (c *Client) handleJoinFrom(user string) {
	logrus.WithField("user", user).Info("Add user to listing")
	c.userList[user] = time.Now().Unix()
}

func (c *Client) handleUserPart(user string) {
	logrus.WithField("user", user).Info("Remove user from listing")
	delete(c.userList, user)
}

func (c *Client) Chatters() []string {
	names := make([]string, 0, len(c.userList))
	for name := range c.userList {
		names = append(names, name)
	}

	return names
}
