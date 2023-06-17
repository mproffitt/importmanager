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

	m "github.com/mproffitt/importmanager/pkg/mime"
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
	Negated    bool
}

func (p *Processor) String() string {
	return fmt.Sprintf("%s (%s)", p.Handler, p.Type)
}

// Path A path object for processors
type Path struct {
	Path       string      `yaml:"path"`
	Processors []Processor `yaml:"processors"`
}

// Config Global config for the application
type Config struct {
	sync.RWMutex
	Paths           []Path        `yaml:"paths"`
	DelayInSeconds  time.Duration `yaml:"delayInSeconds"`
	CleanupZeroByte bool          `yaml:"cleanupZeroByte"`
	PluginPath      string        `yaml:"pluginDirectory"`
	BufferSize      int           `yaml:"bufferSize"`
	LogLevel        string        `yaml:"logLevel"`
	MimeDirectories []string      `yaml:"mimeDirectories"`
	pathHandler     handler
}

// MaxRetries Maximum number of retries for operations
const MaxRetries = 100

// DefaultBufferSize When not set, the jobs buffer will be this size
const DefaultBufferSize = 50

// New Load the config file
func New(configFile string, pathHandler handler) (c *Config, err error) {
	config.pathHandler = pathHandler
	c = &config
	log.SetFormatter(&log.TextFormatter{
		DisableColors: true,
		FullTimestamp: true,
	})
	go watch(context.Background(), configFile)
	err = load(configFile)
	return
}

// IsBuiltIn Test if the given processor is a builtin processor
func IsBuiltIn(plugin string) bool {
	for _, p := range DefaultHandlers {
		if strings.EqualFold(plugin, p) {
			return true
		}
	}
	return false
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		dirname, _ := os.UserHomeDir()
		path = filepath.Join(dirname, path[2:])
	}
	return path
}

func load(filename string) (err error) {
	config.Lock()
	defer config.Unlock()
	pwd, _ := os.Getwd()
	log.Infof("Loading config file %s/%s", pwd, filename)

	var f []byte
	if f, err = ioutil.ReadFile(filename); err != nil {
		return
	}

	if err = yaml.Unmarshal(f, &config); err != nil {
		return
	}

	for i, p := range config.MimeDirectories {
		config.MimeDirectories[i] = expandHome(p)
	}
	m.Load(config.MimeDirectories)

	if config.BufferSize == 0 {
		config.BufferSize = DefaultBufferSize
	}

	config.PluginPath = expandHome(config.PluginPath)

	switch config.LogLevel {
	case "debug":
		log.SetReportCaller(true)
		log.SetLevel(log.DebugLevel)
	case "error":
		log.SetLevel(log.ErrorLevel)
	case "warn":
		log.SetLevel(log.WarnLevel)
	default:
		log.SetLevel(log.InfoLevel)
	}

	for i, p := range config.Paths {
		config.Paths[i].Path = expandHome(p.Path)
		for j, q := range p.Processors {
			if q.Type[0] == '!' {
				q.Type = q.Type[1:]
				q.Negated = true
			}
			config.Paths[i].Processors[j].Path = expandHome(q.Path)
			for k, v := range q.Properties {
				config.Paths[i].Processors[j].Properties[k] = expandHome(v)
			}
			if config.PluginPath != "" && !IsBuiltIn(q.Handler) {
				var handler string = filepath.Join(config.PluginPath, q.Handler)
				if _, err = os.Stat(handler); !os.IsNotExist(err) {
					config.Paths[i].Processors[j].Handler = handler
				}
			}
		}
	}

	DryRun(&config)
	log.Info("Done loading config file")
	return
}

func watch(ctx context.Context, filename string) {
	log.Infof("Setting up watch for config file %s", filename)
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
						data, _ := yaml.Marshal(&config)
						ioutil.WriteFile(filename, data, 0)
						break
					}
					i++
					<-time.After(1 * time.Millisecond)
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
