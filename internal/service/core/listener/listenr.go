package listener

import (
	"context"
	"encoding/hex"
	"fmt"
	"github.com/dl-only-tokens/back-listener/internal/data"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/pkg/errors"
	"gitlab.com/distributed_lab/logan/v3"
	"log"
	"strings"
	"time"
)

type Listener interface {
	Run() error
	HealthCheck() error
	GetNetwork() string
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

			logs, err := client.FilterLogs(context.Background(), query)
			if err != nil {
				return errors.Wrap(err, "failed to filter logs ")
			}

			l.log.Info(fmt.Sprintf("id: %s  %s", l.id, logs))

			if err = l.masterQ.Transaction(l.insertTxs, l.prepareDataToInsert(l.getTxIntputsOnBlock(l.GetTxHashes(logs), block))); err != nil {
				return errors.Wrap(err, "failed to use transaction")
			}

			break
		}

	}
}

func (l *ListenData) getTxIntputsOnBlock(txHashes []common.Hash, block *types.Block) map[string][]string {
	result := make(map[string][]string)

	for _, txHash := range txHashes {
		tx := block.Transaction(txHash)
		l.log.Debug(hex.EncodeToString(tx.Data()))

		parsedData, err := l.parsePailoaderOnInput(hex.EncodeToString(tx.Data()), "") //todo header
		if err != nil {
			return nil
		}
		result[tx.Hash().String()] = parsedData
	}
	return result
}

func (l *ListenData) parsePailoaderOnInput(input string, header string) ([]string, error) { //move to  another class
	index := strings.Index(input, header)
	if index == -1 {
		return nil, errors.New("failed to found substring")
	}
	hexBytes, err := hex.DecodeString(input[index:])
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode string")
	}

	index = strings.Index(string(hexBytes), ".") //todo modify it
	if index == -1 {
		return nil, errors.New("failed to found substring")
	}

	hexBytes, err = hex.DecodeString(string(hexBytes[:index]))
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode string")
	}

	return strings.Split(string(hexBytes), ":"), nil
}

func (l *ListenData) GetTxHashes(logs []types.Log) []common.Hash {
	result := make([]common.Hash, 0)
	for _, event := range logs {
		result = append(result, event.TxHash)
	}

	return result
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

func (l *ListenData) prepareDataToInsert(inputs map[string][]string) []data.Transactions {
	response := make([]data.Transactions, 0)
	for txHash, payload := range inputs {
		response = append(response,
			data.Transactions{
				PaymentID:   payload[1],
				NetworkFrom: payload[2],
				NetworkTo:   payload[3],
				TxHash:      txHash,
				Network:     l.id,
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
