package handler

import (
	"context"
	"github.com/dl-only-tokens/back-listener/internal/config"
	"github.com/dl-only-tokens/back-listener/internal/data"
	"github.com/dl-only-tokens/back-listener/internal/service/core/listener"
	"gitlab.com/distributed_lab/logan/v3"
)

const (
	ChanIsClosed = iota
)

type ListenerHandler struct {
	Listeners       *ListenersMap
	supportNetworks []config.NetInfo
	ctx             context.Context
	log             *logan.Entry
	pauseTime       int
	healthCheckChan chan listener.StateInfo
	rarimoAPI       string
	masterQ         data.MasterQ
	isAutoInit      bool
	txMetaData      *config.MetaData
}
