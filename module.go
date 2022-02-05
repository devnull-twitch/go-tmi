package tmi

import "time"

type (
	ModuleArgs struct {
		Channel   string
		Parameter map[string]string
	}
	Module interface {
		ExternalTrigger(client *Client) <-chan *ModuleArgs
		MessageTrigger(client *Client, incoming *IncomingMessage) *ModuleArgs
		Handler(client *Client, args ModuleArgs) *OutgoingMessage
	}
	ModCommandHandler func(client *Client, m Module, args CommandArgs) *OutgoingMessage
	ModuleCommand     struct {
		Command
		ModuleCommandHandler ModCommandHandler
	}
)

func CreateTimeTrigger(interval time.Duration) <-chan *ModuleArgs {
	c := make(chan *ModuleArgs)
	go func() {
		for {
			time.Sleep(interval)
			c <- &ModuleArgs{}
		}
	}()

	return c
}

func WrapTriggerCondition(trigger <-chan *ModuleArgs, checkFn func(*ModuleArgs) bool) <-chan *ModuleArgs {
	filtered := make(chan *ModuleArgs)
	go func() {
		for {
			event := <-trigger
			if checkFn(event) {
				filtered <- event
			}
		}
	}()

	return filtered
}

func (c *Client) AddModuleCommand(m Module, mcmd ModuleCommand) {
	mcmd.Command.Handler = func(client *Client, args CommandArgs) *OutgoingMessage {
		return mcmd.ModuleCommandHandler(client, m, args)
	}
	c.AddCommand(mcmd.Command)
}

func (c *Client) AddModule(m Module) {
	triggerChan := m.ExternalTrigger(c)
	go func() {
		for {
			args := <-triggerChan
			if args != nil {
				out := m.Handler(c, *args)
				if out != nil {
					c.Send(out)
				}
			}
		}
	}()

	c.modules = append(c.modules, m)
}
