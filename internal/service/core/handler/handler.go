package handler

import (
	"context"
	"encoding/json"
	"github.com/dl-only-tokens/back-listener/internal/config"
	"github.com/dl-only-tokens/back-listener/internal/data"
	"github.com/dl-only-tokens/back-listener/internal/service/core/listener"
	"github.com/dl-only-tokens/back-listener/internal/service/core/rarimo"
	"github.com/pkg/errors"
	"gitlab.com/distributed_lab/logan/v3"
	"net/http"
)

type Handler interface {
	Run()
	InitListeners() error
	healthCheck()
	getContractsAddresses() ([]rarimo.Data, error)
	prepareNewListener(network string, address string) (listener.Listener, error)
	addNewListener(listener listener.Listener)
}

func NewHandler(log *logan.Entry, networker []config.NetInfo, api string, masterQ data.MasterQ) Handler {
	return &ListenerHandler{
		Listeners: NewCounters(),
		ctx:       context.Background(),
		log:       log,
		Networker: networker,
		pauseTime: 2, //todo  remove magic number
		rarimoAPI: api,
		masterQ:   masterQ,
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

func (h *ListenerHandler) healthCheck() {
	for {
		select {
		case chanStatus := <-h.healthCheckChan:
			h.Listeners.Store(h.findByNetwork(chanStatus.Name), false)
		}
	}
}

func (h *ListenerHandler) InitListeners() error {
	networks, err := h.getContractsAddresses()
	if err != nil {
		return errors.Wrap(err, "failed to get contract list")
	}
	for _, network := range networks {
		preparedListener, err := h.prepareNewListener(network.Attributes.Name, network.Attributes.BridgeContract)
		if err != nil {
			h.log.WithError(err).Error("failed to  connect to  rpc")
			continue
		}

		h.addNewListener(preparedListener)
	}
	return nil
}

func (h *ListenerHandler) getContractsAddresses() ([]rarimo.Data, error) {
	resp, err := http.Get(h.rarimoAPI)
	if err != nil {
		return nil, errors.Wrap(err, "failed to send request")
	}

	if resp.StatusCode >= 300 && resp.StatusCode < 500 {
		return nil, errors.New("bad response code")
	}

	decodedResponse := new(rarimo.NetworkListResponse)

	if err := json.NewDecoder(resp.Body).Decode(&decodedResponse); err != nil {
		return nil, errors.New("failed to decode response ")
	}
	return decodedResponse.Data, nil
}

func (h *ListenerHandler) prepareNewListener(network string, address string) (listener.Listener, error) {
	netInfo := h.findNetwork(network)
	if netInfo == nil {
		return nil, errors.New("unsupported network")
	}

	info := listener.EthInfo{
		Address:     address,
		RPC:         netInfo.Rpc,
		NetworkName: network,
	}

	return listener.NewListener(h.log, h.pauseTime, info, h.masterQ), nil
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
	for _, l := range h.Networker {
		if l.Name == network {
			return &l
		}
	}
	return nil
}
