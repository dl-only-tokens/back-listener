package listener

import (
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/pkg/errors"
	"gitlab.com/distributed_lab/logan/v3"
	"log"
	"sync"
	"time"
)

type Listener interface {
	Run(wg *sync.WaitGroup) error
	HealthCheck() error
}

type ListenData struct {
	id        string
	log       *logan.Entry
	pauseTime int
	ctx       context.Context
	isActive  bool
	rpc       string
	address   string
}

type EthInfo struct {
	Address     string
	RPC         string
	NetworkName string
}

func NewListener(log *logan.Entry, pauseTime int, ethInfo EthInfo) Listener {
	return &ListenData{
		id:        ethInfo.NetworkName,
		log:       log,
		pauseTime: pauseTime,
		ctx:       context.Background(),
		rpc:       ethInfo.RPC,
		address:   ethInfo.Address,

		//masterQ
	}
}

func (l *ListenData) Run(wg *sync.WaitGroup) error {
	l.isActive = true
	defer wg.Done()

	client, err := ethclient.Dial(l.rpc)
	if err != nil {
		return errors.Wrap(err, "failed to connect to node")
	}

	//todo get list  of addresses from api and look  for this networks on the config
	contractAddress := common.HexToAddress(l.address)
	if err != nil {
		return errors.Wrap(err, "failed to prepare address")
	}

	var previewHash common.Hash

	ticker := time.NewTicker(time.Duration(l.pauseTime) * time.Second)
	for {
		select {
		case <-l.ctx.Done():
			l.isActive = false
			return nil
		case <-ticker.C:
			//todo move to external  func
			block, err := client.BlockByNumber(context.Background(), nil)
			if err != nil {
				return errors.Wrap(err, "failed to get last block ")
			}
			hash := block.Hash()

			if previewHash == hash {
				continue
			}
			log.Println("--------------------------------------------")
			query := ethereum.FilterQuery{
				BlockHash: &hash,
				Addresses: []common.Address{contractAddress},
			}
			previewHash = hash

			sub, err := client.FilterLogs(context.Background(), query)
			if err != nil {
				return errors.Wrap(err, "failed to filter logs ")
			}

			l.log.Info(fmt.Sprintf("id: %s  %s", l.id, sub))
			break
		}

	}
}

func (l *ListenData) HealthCheck() error {
	if !l.isActive {
		return errors.New("lister isn't active")
	}

	return nil
}
