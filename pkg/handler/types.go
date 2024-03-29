package handler

import (
	"sync"
	"time"

	c "github.com/mproffitt/importmanager/pkg/config"
	"github.com/rjeczalik/notify"
)

type event struct {
	event notify.Event
	time  time.Time
}

type watch struct {
	stop     chan bool
	complete chan bool
	events   chan notify.EventInfo
}

type job struct {
	path       string
	processors []c.Processor
	czb        bool
	ready      bool
	complete   chan bool
}

type lockable struct {
	sync.RWMutex
	paths map[string]event
}
