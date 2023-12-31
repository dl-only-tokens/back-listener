package handler

import (
	"context"
	"github.com/dl-only-tokens/back-listener/internal/config"
	"github.com/dl-only-tokens/back-listener/internal/data"
	"github.com/dl-only-tokens/back-listener/internal/service/core/listener"
	"gitlab.com/distributed_lab/logan/v3"
)

type ListenerHandler struct {
	Listeners       []listener.Listener
	supportNetworks []config.NetInfo
	ctx             context.Context
	log             *logan.Entry
	pauseTime       int
	healthCheckChan chan listener.Listener
	rarimoAPI       string
	masterQ         data.MasterQ
	txMetaData      *config.MetaData
	abiPath         string
}
