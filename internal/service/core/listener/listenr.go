package listener

import (
	"context"
	"fmt"
	"github.com/dl-only-tokens/back-listener/internal/data"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/pkg/errors"
	"gitlab.com/distributed_lab/logan/v3"
	"log"
	"time"
)

type Listener interface {
	Run() error
	HealthCheck() error
	GetNetwork() string
	decodeData(log types.Log) []byte
	filterByMetaData(logs []types.Log) []types.Log
}

type ListenData struct {
	id        string
	log       *logan.Entry
	pauseTime int
	ctx       context.Context
	isActive  bool
	rpc       string
	address   string
	masterQ   data.MasterQ
}

type EthInfo struct {
	Address     string
	RPC         string
	NetworkName string
}

func NewListener(log *logan.Entry, pauseTime int, ethInfo EthInfo, masterQ data.MasterQ) Listener {
	return &ListenData{
		id:        ethInfo.NetworkName,
		log:       log,
		pauseTime: pauseTime,
		ctx:       context.Background(),
		rpc:       ethInfo.RPC,
		address:   ethInfo.Address,
		masterQ:   masterQ,
	}
}

func (l *ListenData) Run() error {
	l.isActive = true

	client, err := ethclient.Dial(l.rpc)
	if err != nil {
		return errors.Wrap(err, "failed to connect to node")
	}

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
			log.Println("--------------------------------------------") //todo  remove  it
			query := ethereum.FilterQuery{
				BlockHash: &hash,
				Addresses: []common.Address{contractAddress},
			}
			previewHash = hash

			sub, err := client.FilterLogs(context.Background(), query)
			if err != nil {
				return errors.Wrap(err, "failed to filter logs ")
			}

			sub = l.filterByMetaData(sub)

			l.log.Info(fmt.Sprintf("id: %s  %s", l.id, sub))

			if err := l.masterQ.Transaction(l.insertTxs, l.prepareDataToInsert(sub)); err != nil {
				return errors.Wrap(err, "failed to use transaction")
			}
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

func (l *ListenData) GetNetwork() string {
	return l.id
}

func (l *ListenData) filterByMetaData(logs []types.Log) []types.Log {
	for _, log := range logs {
		//todo  decode data and  filter its
		l.decodeData(log)
	}
	return logs
}

func (l *ListenData) decodeData(log types.Log) []byte {
	//todo  implement decoder
	return log.Data
}

func (l *ListenData) prepareDataToInsert(logs []types.Log) []data.Transactions {
	response := make([]data.Transactions, 0)
	for _, event := range logs {
		response = append(response,
			data.Transactions{
				PaymentID: "",
				Recipient: event.Address.Hex(),
				TxHash:    event.TxHash.String(),
				Network:   l.id,
			})
	}

	return response
}

func (l *ListenData) insertTxs(any interface{}) error {
	txs, err := l.interfaceToTx(any)
	if err != nil {
		return errors.Wrap(err, "failed tom decode data")
	}
	for _, tx := range *txs {
		if err := l.masterQ.TransactionsQ().New().Insert(&tx); err != nil {
			return errors.Wrap(err, "failed to  insert tx to db")
		}
	}
	return nil
}

func (l *ListenData) interfaceToTx(any interface{}) (*[]data.Transactions, error) {
	if convertedData, ok := any.([]data.Transactions); ok {
		return &convertedData, nil
	}

	return nil, errors.New("data cant be converted to KYCWebhookResponse")
}
