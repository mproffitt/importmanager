package config

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	m "github.com/mproffitt/importmanager/pkg/mime"
	log "github.com/sirupsen/logrus"
	yaml "gopkg.in/yaml.v2"
)

type defaultHandlers []string

// DefaultHandlers a set of valid handler
var DefaultHandlers defaultHandlers = defaultHandlers{
	"copy",
	"move",
	"extract",
	"install",
	"delete",
}

// IsBuiltIn Test if the given processor is a builtin processor
func (d *defaultHandlers) IsBuiltIn(plugin string) bool {
	for _, p := range *d {
		if strings.EqualFold(plugin, p) {
			return true
		}
	}
	return false
}

func (p *Processor) String() string {
	return fmt.Sprintf("%s (%s)", p.Handler, p.Type)
}

// MaxRetries Maximum number of retries for operations
const MaxRetries = 100

// DefaultBufferSize When not set, the jobs buffer will be this size
const DefaultBufferSize = 50

// New Create a new Config object
//
// Arguments:
//
// - configFile  string  The full path to the config file to load
// - pathHandler handler A custom function of type handler to call during dry-run
// - autoReload  bool    If true, sets up watches on the config file and  automatically reloads it when it changes
//
// Return:
//
// - *Config A pointer to the loaded configuration
// - error   The last error which occured during loading
func New(configFile string, pathHandler handler, autoReload bool) (c *Config, err error) {
	c = &Config{
		pathHandler: pathHandler,
	}

	log.SetFormatter(&log.TextFormatter{
		DisableColors: true,
		FullTimestamp: true,
	})
	if autoReload {
		go c.watch(context.Background(), configFile)
	}
	err = c.load(configFile)
	return
}

func expandHome(path *string) {
	var p string = (*path)
	if p[0] == '~' && p[1] != '/' {
		p = "~/" + p[1:]
	}

	if p[:2] == "~/" {
		dirname, _ := os.UserHomeDir()
		p = filepath.Join(dirname, p[2:])
	}

	*path = p
	return
}

func (c *Config) load(filename string) (err error) {
	c.Lock()
	defer c.Unlock()
	pwd, _ := os.Getwd()
	log.Infof("Loading config file %s/%s", pwd, filename)

	var f []byte
	if f, err = ioutil.ReadFile(filename); err != nil {
		return
	}

	if err = yaml.Unmarshal(f, c); err != nil {
		return
	}

	for i := range c.MimeDirectories {
		expandHome(&c.MimeDirectories[i])
	}
	m.Load(c.MimeDirectories)

	if c.BufferSize == 0 {
		c.BufferSize = DefaultBufferSize
	}

	expandHome(&c.PluginPath)

	c.setupLogging()
	for i, p := range c.Paths {
		expandHome(&c.Paths[i].Path)
		for j, q := range p.Processors {
			if q.Type[0] == '!' {
				q.Type = q.Type[1:]
				q.Negated = true
			}
			expandHome(&c.Paths[i].Processors[j].Path)
			for k, v := range q.Properties {
				expandHome(&v)
				c.Paths[i].Processors[j].Properties[k] = v
			}
			if c.PluginPath != "" && !DefaultHandlers.IsBuiltIn(q.Handler) {
				var handler string = filepath.Join(c.PluginPath, q.Handler)
				if _, err = os.Stat(handler); !os.IsNotExist(err) {
					c.Paths[i].Processors[j].Handler = handler
				}
			}
		}
	}

	// log.Info("Starting dry run validation")
	// DryRun(c)
	log.Info("Done loading config file")
	return
}

func (c *Config) setupLogging() {
	switch c.LogLevel {
	case "trace":
		fallthrough
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
}
