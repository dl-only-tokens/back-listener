package handler

import (
	"context"
	"fmt"
	"github.com/dl-only-tokens/back-listener/internal/config"
	"github.com/dl-only-tokens/back-listener/internal/data"
	"github.com/dl-only-tokens/back-listener/internal/service/core/listener"
	"github.com/pkg/errors"
	"gitlab.com/distributed_lab/logan/v3"
	"math/big"
	"sync"
)

type Handler interface {
	Run()
	Init() error
}

func NewHandler(log *logan.Entry, networker []config.NetInfo, masterQ data.MasterQ, metaData *config.MetaData, chainListener *config.ChainListener) Handler {
	return &ListenerHandler{
		Listeners:       make([]listener.Listener, 0),
		ctx:             context.Background(),
		log:             log,
		supportNetworks: networker,
		pauseTime:       chainListener.PauseTime,
		masterQ:         masterQ,
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
	if err := h.initListeners(h.supportNetworks); err != nil {
		return errors.Wrap(err, "failed to do auto  init")
	}

	return nil

}

func (h *ListenerHandler) initListeners(data []config.NetInfo) error {
	for _, network := range data {

		preparedListener, err := h.prepareNewListener(network.Name, false)
		if err != nil {
			h.log.WithError(err).Error("failed to connect to  rpc")
			continue
		}

		h.addNewListener(preparedListener)

		preparedListener, err = h.prepareNewListener(network.Name, true)
		if err != nil {
			h.log.WithError(err).Error("failed to connect to  rpc")
			continue
		}

		h.addNewListener(preparedListener)
	}

	return nil
}

func (h *ListenerHandler) prepareNewListener(network string, isIndexer bool) (listener.Listener, error) {
	netInfo := h.findNetwork(network)
	if netInfo == nil {
		return nil, errors.New(fmt.Sprintf("unsupported network: %s", network))
	}

	info := listener.EthInfo{
		RPC:         netInfo.Rpc,
		ChainID:     netInfo.ChainID,
		NetworkName: network,
	}

	var startBlock *big.Int

	if isIndexer {
		network = fmt.Sprint(network, "_indexer")
		startBlock = big.NewInt(int64(netInfo.StartBlock))
	}
	return listener.NewListener(h.ctx, h.log, h.pauseTime, info, h.masterQ, h.txMetaData, h.healthCheckChan, h.abiPath, startBlock, isIndexer), nil
}

func (h *ListenerHandler) addNewListener(listener listener.Listener) {
	h.Listeners = append(h.Listeners, listener)
}

func (h *ListenerHandler) healthCheck() {
	for {
		failedListeners := <-h.healthCheckChan
		if failedListeners.IsIndexer() {
			continue
		}

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
