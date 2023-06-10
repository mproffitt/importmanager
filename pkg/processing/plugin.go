package processing

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	c "github.com/mproffitt/importmanager/pkg/config"
	m "github.com/mproffitt/importmanager/pkg/mime"
	log "github.com/sirupsen/logrus"
)

type arguments struct {
	Source      string            `json:"source"`
	Destination string            `json:"destination"`
	Details     m.Details         `json:"details"`
	Properties  map[string]string `json:"properties"`
}

func toArgumentJSON(source, dest string, details *m.Details, properties map[string]string) string {
	var a arguments = arguments{
		Source:      source,
		Destination: dest,
		Details:     *details,
		Properties:  properties,
	}

	b, _ := json.Marshal(a)
	return string(b)
}

func runPlugin(source, dest string, details *m.Details, processor *c.Processor) (err error) {
	if _, err = os.Stat(processor.Handler); os.IsNotExist(err) {
		err = fmt.Errorf("Plugin file has been moved or deleted from disk. %s", err)
		return
	}

	var (
		executable string
		args       string = toArgumentJSON(source, dest, details, processor.Properties)
		response   []byte
	)

	switch strings.ToLower(filepath.Ext(processor.Handler)) {
	case ".py":
		executable = "python"
	case ".sh":
		executable = "sh"
	case ".bash":
		executable = "bash"
	default:
		err = fmt.Errorf("Invalid plugin filetype")
		return
	}

	cmd := exec.Command(executable, []string{processor.Handler, args}...)
	log.Infof("Triggering plugin command: %s", cmd.String())
	if response, err = cmd.CombinedOutput(); err != nil {
		err = fmt.Errorf("Error in plugin %s: %s - %s", processor.Handler, err.Error(), string(response))
		return
	}
	log.Infof("%s: %s", processor.Handler, string(response))
	return
}
