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
	"sync"
	"time"
)

type Handler interface {
	Run()
	InitListeners() error
	healthCheck() error
	getContractsAddresses() ([]rarimo.Data, error)
	prepareNewListener(network string, address string) (listener.Listener, error)
	addNewListener(listener listener.Listener)
}

type ListenerHandler struct { //todo  rename
	Listeners       []listener.Listener
	ctx             context.Context
	log             *logan.Entry
	pauseTime       int
	healthCheckTime int64
	rarimoAPI       string
	networker       []config.NetInfo
	masterQ         data.MasterQ
}

func NewHandler(log *logan.Entry, networker []config.NetInfo, api string, masterQ data.MasterQ) Handler {
	return &ListenerHandler{
		Listeners: make([]listener.Listener, 0),
		ctx:       context.Background(),
		log:       log,
		networker: networker,
		//todo add time config
		pauseTime: 2,
		rarimoAPI: api,
		masterQ:   masterQ,
	}
}

func (h *ListenerHandler) Run() {
	wg := new(sync.WaitGroup)
	for _, l := range h.Listeners { // todo  make better name
		wg.Add(1)
		go l.Run(wg)
	}
	wg.Wait()
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

func (h *ListenerHandler) healthCheck() error {
	ticker := time.NewTicker(time.Duration(h.healthCheckTime) * time.Second)

	select {
	case <-ticker.C:
		for _, l := range h.Listeners {
			if err := l.HealthCheck(); err != nil {
				return errors.Wrap(err, "failed to check health")
			}
		}
		ticker.Reset(time.Duration(h.healthCheckTime) * time.Second)
	default:
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

func (h *ListenerHandler) addNewListener(listener listener.Listener) {
	h.Listeners = append(h.Listeners, listener)
}

func (h *ListenerHandler) findNetwork(network string) *config.NetInfo {
	for _, l := range h.networker {
		if l.Name == network {
			return &l
		}
	}
	return nil
}
