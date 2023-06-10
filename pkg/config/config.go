package config

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	n "github.com/rjeczalik/notify"
	log "github.com/sirupsen/logrus"
	yaml "gopkg.in/yaml.v2"
)

// DefaultHandlers a set of valid handler
var DefaultHandlers []string = []string{
	"copy",
	"move",
	"extract",
	"install",
	"delete",
}

var config Config

// Processor How to handle a particular file type
type Processor struct {
	Type       string            `yaml:"type"`
	Path       string            `yaml:"path"`
	Handler    string            `yaml:"handler"`
	Properties map[string]string `yaml:"properties"`
}

func (p *Processor) String() string {
	return fmt.Sprintf("%s (%s)", p.Handler, p.Type)
}

// Config Global config for the application
type Config struct {
	sync.RWMutex
	Watch           []string      `yaml:"watch"`
	Processors      []Processor   `yaml:"processors"`
	DelayInSeconds  time.Duration `yaml:"delayInSeconds"`
	CleanupZeroByte bool          `yaml:"cleanupZeroByte"`
	PluginPath      string        `yaml:"pluginDirectory"`
	BufferSize      int           `yaml:"bufferSize"`
	LogLevel        string        `yaml:"logLevel"`
}

// MaxRetries Maximum number of retries for operations
const MaxRetries = 100

// DefaultBufferSize When not set, the jobs buffer will be this size
const DefaultBufferSize = 50

// New Load the config file
func New(configFile string) (c *Config, err error) {
	c = &config
	go watch(context.Background(), configFile)
	err = load(configFile)
	return
}

func load(filename string) (err error) {
	config.RLock()
	defer config.RUnlock()
	log.Info("Loading config file")

	var f []byte
	if f, err = ioutil.ReadFile(filename); err != nil {
		return
	}

	if err = yaml.Unmarshal(f, &config); err != nil {
		return
	}

	if config.BufferSize == 0 {
		config.BufferSize = DefaultBufferSize
	}

	switch config.LogLevel {
	case "debug":
		log.SetLevel(log.DebugLevel)
	case "error":
		log.SetLevel(log.ErrorLevel)
	case "warn":
		log.SetLevel(log.WarnLevel)
	default:
		log.SetLevel(log.InfoLevel)
	}

	dirname, _ := os.UserHomeDir()
	for i, p := range config.Watch {
		if strings.HasPrefix(p, "~/") {
			config.Watch[i] = filepath.Join(dirname, p[2:])
			log.Debugf("Path %s became %s", p, config.Watch[i])
		}
	}

	if strings.HasPrefix(config.PluginPath, "~/") {
		config.PluginPath = filepath.Join(dirname, config.PluginPath[2:])
	}

	for i, p := range config.Processors {
		if strings.HasPrefix(p.Path, "~/") {
			config.Processors[i].Path = filepath.Join(dirname, p.Path[2:])
			log.Debugf("Path %s became %s", p.Path, config.Processors[i].Path)
		}
		if config.PluginPath != "" {
			var handler string = filepath.Join(config.PluginPath, p.Handler)
			if _, err = os.Stat(handler); !os.IsNotExist(err) {
				config.Processors[i].Handler = handler
			}
		}
	}
	log.Info("Done loading config file")
	return
}

func watch(ctx context.Context, filename string) {
	events := n.Remove | n.Write | n.InModify | n.InCloseWrite
	c := make(chan n.EventInfo, 1)
	if err := n.Watch(filename, c, events); err != nil {
		log.Fatal(err)
	}
	defer n.Stop(c)

	for {
		select {
		case <-ctx.Done():
			return

		case ei := <-c:
			switch ei.Event() {
			// VIM is a special case and renames / removes the old buffer
			// and recreates a new one in place. This means we need to
			// set up a new watch on the file to ensure we track further
			// updates to it.
			case n.Remove:
				var i int = 0
				for {
					if _, err := os.Stat(filename); err == nil {
						break
					}
					if i == MaxRetries {
						// If we got here and the config wasn't recreted
						// create it with the last known config values
						data, _ := yaml.Marshal(&config)
						ioutil.WriteFile(filename, data, 0)
						break
					}
					i++
					time.Sleep(1 * time.Millisecond)
				}
				n.Stop(c)
				if err := n.Watch(filename, c, events); err != nil {
					log.Println(err)
				}
				defer n.Stop(c)
				fallthrough
			case n.Write:
				fallthrough
			case n.InModify:
				fallthrough
			case n.InCloseWrite:
				if err := load(filename); err != nil {
					log.Fatal("Unable to load config file", err)
					return
				}
			}
		}
	}
}
