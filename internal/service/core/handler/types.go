package handler

import (
	"context"
	"github.com/dl-only-tokens/back-listener/internal/config"
	"github.com/dl-only-tokens/back-listener/internal/data"
	"gitlab.com/distributed_lab/logan/v3"
	"sync"
)

const (
	ChanIsClosed = iota
)

type ListenerHandler struct { //todo  rename
	Listeners       *ListenersMap
	Networker       []config.NetInfo
	ctx             context.Context
	test            sync.Map
	log             *logan.Entry
	pauseTime       int
	healthCheckChan chan StateInfo
	rarimoAPI       string
	masterQ         data.MasterQ
	isAutoInit      bool
}

type StateInfo struct {
	Name   string
	Status string
}
