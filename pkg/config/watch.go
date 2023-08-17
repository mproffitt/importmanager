package config

import (
	"context"
	"io/ioutil"
	"os"
	"time"

	n "github.com/rjeczalik/notify"
	log "github.com/sirupsen/logrus"
	yaml "gopkg.in/yaml.v2"
)

func (c *Config) watch(ctx context.Context, filename string) {
	log.Infof("Setting up watch for config file %s", filename)
	events := n.Remove | n.Write | n.InModify | n.InCloseWrite
	channel := make(chan n.EventInfo, 1)
	if err := n.Watch(filename, channel, events); err != nil {
		log.Fatal(err)
	}
	defer n.Stop(channel)

	for {
		select {
		case <-ctx.Done():
			return

		case ei := <-channel:
			switch ei.Event() {
			// VIM is a special case and renames / removes the old buffer
			// and recreates a new one in place. This means we need to
			// set up a new watch on the file to ensure we track further
			// updates to it.
			case n.Rename:
				fallthrough
			case n.Remove:
				var i int = 0
				for {
					if _, err := os.Stat(filename); err == nil {
						break
					}
					if i == MaxRetries {
						// If we got here and the config wasn't recreted
						// create it with the last known config values
						data, _ := yaml.Marshal(c)
						ioutil.WriteFile(filename, data, 0)
						break
					}
					i++
					<-time.After(1 * time.Millisecond)
				}
				n.Stop(channel)
				if err := n.Watch(filename, channel, events); err != nil {
					log.Println(err)
				}
				defer n.Stop(channel)
				fallthrough
			case n.Write:
				fallthrough
			case n.InModify:
				fallthrough
			case n.InCloseWrite:
				if err := c.load(filename); err != nil {
					log.Fatal("Unable to load config file", err)
					return
				}
			}
		}
	}
}
