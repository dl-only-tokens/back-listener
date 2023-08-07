package handler

import (
	"context"
	"github.com/dl-only-tokens/back-listener/internal/config"
	"github.com/dl-only-tokens/back-listener/internal/data"
	"github.com/dl-only-tokens/back-listener/internal/service/core/listener"
	"github.com/dl-only-tokens/back-listener/internal/service/core/rarimo"
	"github.com/pkg/errors"
	"gitlab.com/distributed_lab/logan/v3"
)

type Handler interface {
	Run()
	Init() error
	healthCheck()
	autoInitContracts() error
	initListeners(data []config.NetInfo) error
	findByNetwork(network string) listener.Listener
	prepareNewListener(network string, address string) (listener.Listener, error)
	addNewListener(listener listener.Listener)
	findNetwork(network string) *config.NetInfo
}

func NewHandler(log *logan.Entry, networker []config.NetInfo, rarimoApi *config.API, masterQ data.MasterQ) Handler {
	return &ListenerHandler{
		Listeners:       NewCounters(),
		ctx:             context.Background(),
		log:             log,
		supportNetworks: networker,
		pauseTime:       2, //todo  remove magic number
		rarimoAPI:       rarimoApi.Endpoint,
		masterQ:         masterQ,
		isAutoInit:      rarimoApi.IsAutoInit,
	}
}

func (h *ListenerHandler) Run() {
	go h.healthCheck()

	for {
		for l, status := range h.Listeners.data { // todo make better name
			if !status {
				go l.Run()
				h.Listeners.Store(l, true)
			}
		}
	}

}

func (h *ListenerHandler) Init() error {

	if h.isAutoInit {
		if err := h.autoInitContracts(); err != nil {
			return errors.Wrap(err, "failed to do auto  init")
		}
		return nil
	}

	if err := h.initListeners(h.supportNetworks); err != nil {
		return errors.Wrap(err, "failed to do auto  init")
	}

	return nil

}

func (h *ListenerHandler) autoInitContracts() error {
	rarimoHandler := rarimo.NewRarimoHandler(h.rarimoAPI)
	networks, err := rarimoHandler.GetContractsAddresses()
	if err != nil {
		return errors.Wrap(err, "failed to get contract list")
	}

	if err = h.initListeners(networks); err != nil {
		return errors.Wrap(err, "failed to initListeners")
	}

	return nil
}

func (h *ListenerHandler) initListeners(data []config.NetInfo) error {
	for _, network := range data {
		preparedListener, err := h.prepareNewListener(network.Name, network.Address)
		if err != nil {
			h.log.WithError(err).Error("failed to connect to  rpc")
			continue
		}

		h.addNewListener(preparedListener)
	}
	return nil
}

func (h *ListenerHandler) prepareNewListener(network string, address string) (listener.Listener, error) {
	netInfo := h.findNetwork(network)
	if netInfo == nil {
		return nil, errors.New("unsupported network")
	}

	if len(address) == 0 {
		return nil, errors.New("address is empty")
	}

	info := listener.EthInfo{
		Address:     address,
		RPC:         netInfo.Rpc,
		NetworkName: network,
	}

	return listener.NewListener(h.log, h.pauseTime, info, h.masterQ), nil
}

func (h *ListenerHandler) healthCheck() {
	for {
		select {
		case chanStatus := <-h.healthCheckChan:
			h.Listeners.Store(h.findByNetwork(chanStatus.Name), false)
		}
	}
}

func (h *ListenerHandler) findByNetwork(network string) listener.Listener {
	for l := range h.Listeners.data {
		if l.GetNetwork() == network {
			return l
		}
	}
	return nil
}

func (h *ListenerHandler) addNewListener(listener listener.Listener) {
	h.Listeners.Store(listener, false)
}

func (h *ListenerHandler) findNetwork(network string) *config.NetInfo {
	for _, l := range h.supportNetworks {
		if l.Name == network {
			return &l
		}
	}
	return nil
}
