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

// DefaultHandlers a set of valid handler
var DefaultHandlers []string = []string{
	"copy",
	"move",
	"extract",
	"install",
	"delete",
}

var config Config

func (p *Processor) String() string {
	return fmt.Sprintf("%s (%s)", p.Handler, p.Type)
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

	for i := range config.MimeDirectories {
		expandHome(&config.MimeDirectories[i])
	}
	m.Load(config.MimeDirectories)

	if config.BufferSize == 0 {
		config.BufferSize = DefaultBufferSize
	}

	expandHome(&config.PluginPath)

	switch config.LogLevel {
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

	for i, p := range config.Paths {
		expandHome(&config.Paths[i].Path)
		for j, q := range p.Processors {
			if q.Type[0] == '!' {
				q.Type = q.Type[1:]
				q.Negated = true
			}
			expandHome(&config.Paths[i].Processors[j].Path)
			for k, v := range q.Properties {
				expandHome(&v)
				config.Paths[i].Processors[j].Properties[k] = v
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
