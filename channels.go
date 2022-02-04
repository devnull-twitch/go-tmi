package tmi

import "fmt"

func (c *Client) JoinChannel(channel string) {
	c.ircClient.Write(fmt.Sprintf("JOIN #%s", channel))
}

func (c *Client) LeaveChannel(channel string) {
	c.ircClient.Write(fmt.Sprintf("PART #%s", channel))
}
