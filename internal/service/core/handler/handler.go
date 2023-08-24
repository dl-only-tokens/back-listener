package handler

import (
	"context"
	"fmt"
	"github.com/dl-only-tokens/back-listener/internal/config"
	"github.com/dl-only-tokens/back-listener/internal/data"
	"github.com/dl-only-tokens/back-listener/internal/service/core/listener"
	"github.com/dl-only-tokens/back-listener/internal/service/core/rarimo"
	"github.com/pkg/errors"
	"gitlab.com/distributed_lab/logan/v3"
	"sync"
)

type Handler interface {
	Run()
	Init() error
}

func NewHandler(log *logan.Entry, networker []config.NetInfo, rarimoApi *config.API, masterQ data.MasterQ, metaData *config.MetaData, chainListener *config.ChainListener) Handler {
	return &ListenerHandler{
		Listeners:       make([]listener.Listener, 0),
		ctx:             context.Background(),
		log:             log,
		supportNetworks: networker,
		pauseTime:       chainListener.PauseTime,
		rarimoAPI:       rarimoApi.Endpoint,
		masterQ:         masterQ,
		isAutoInit:      rarimoApi.IsAutoInit,
		txMetaData:      metaData,
		abiPath:         chainListener.AbiPath,
		healthCheckChan: make(chan listener.Listener),
	}
}

func (h *ListenerHandler) Run() {
	wg := new(sync.WaitGroup)
	wg.Add(1)

	for _, l := range h.Listeners {
		go l.Restart(h.ctx)
	}

	go h.healthCheck()

	wg.Wait()
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
		return nil, errors.New(fmt.Sprintf("unsupported network: %s", network))
	}

	if len(address) == 0 {
		return nil, errors.New("address is empty")
	}

	info := listener.EthInfo{
		Address:     address,
		RPC:         netInfo.Rpc,
		ChainID:     netInfo.ChainID,
		NetworkName: network,
	}

	return listener.NewListener(h.ctx, h.log, h.pauseTime, info, h.masterQ, h.txMetaData, h.healthCheckChan, h.abiPath, nil), nil
}

func (h *ListenerHandler) addNewListener(listener listener.Listener) {
	h.Listeners = append(h.Listeners, listener)
}

func (h *ListenerHandler) healthCheck() {
	for {

		failedListeners := <-h.healthCheckChan
		go failedListeners.Restart(h.ctx)
	}
}

func (h *ListenerHandler) findNetwork(network string) *config.NetInfo {
	for _, l := range h.supportNetworks {
		if l.Name == network {
			return &l
		}
	}

	return nil
}
