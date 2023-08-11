package listener

import (
	"bytes"
	"context"
	"encoding/hex"
	"github.com/dl-only-tokens/back-listener/internal/config"
	"github.com/dl-only-tokens/back-listener/internal/data"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/pkg/errors"
	"gitlab.com/distributed_lab/logan/v3"
	"os"
	"strings"
	"time"
)

type Listener interface {
	Run()
	GetNetwork() string
}

type ListenData struct {
	id              string
	log             *logan.Entry
	pauseTime       int
	ctx             context.Context
	rpc             string
	address         string
	masterQ         data.MasterQ
	txMetaData      *config.MetaData
	healthCheckChan chan StateInfo
}

type EthInfo struct {
	Address     string
	RPC         string
	NetworkName string
}

func NewListener(log *logan.Entry, pauseTime int, ethInfo EthInfo, masterQ data.MasterQ, metaData *config.MetaData, healthCheckChan chan StateInfo) Listener {
	return &ListenData{
		id:              ethInfo.NetworkName,
		log:             log,
		pauseTime:       pauseTime,
		ctx:             context.Background(),
		rpc:             ethInfo.RPC,
		address:         ethInfo.Address,
		masterQ:         masterQ,
		txMetaData:      metaData,
		healthCheckChan: healthCheckChan,
	}
}

func (l *ListenData) Run() {
	client, err := ethclient.Dial(l.rpc)
	if err != nil {

		l.log.WithError(err).Error("failed to connect to node")
		return
	}

	contractAddress := common.HexToAddress(l.address)
	if err != nil {
		l.log.WithError(err).Error("failed to prepare address")
		return
	}

	var previewHash common.Hash

	ticker := time.NewTicker(time.Duration(l.pauseTime) * time.Second)
	for {
		select {
		case <-l.ctx.Done():
			l.healthCheckChan <- StateInfo{
				Name: l.id,
			}
			return
		case <-ticker.C:
			block, err := client.BlockByNumber(context.Background(), nil)
			if err != nil {
				l.log.WithError(err).Error("failed to get last block ")
				return
			}

			hash := block.Hash()
			if previewHash == hash {
				continue
			}
			query := ethereum.FilterQuery{
				BlockHash: &hash,
				Addresses: []common.Address{contractAddress},
			}
			previewHash = hash

			logs, err := client.FilterLogs(context.Background(), query)
			if err != nil {
				continue
			}

			suitableTXs, err := l.parseRecipientFromEvent(logs)
			if err != nil {
				l.log.WithError(err).Error("failed to  get suitable")
				continue
			}
			if err = l.insertTxs(l.prepareDataToInsert(l.getTxIntputsOnBlock(suitableTXs, block))); err != nil {
				l.log.Error(errors.Wrap(err, "failed to use transaction"))
				continue
			}

			break
		}

	}
}

func (l *ListenData) getTxIntputsOnBlock(txHashes []RecipientInfo, block *types.Block) map[string][]string {
	result := make(map[string][]string)

	for _, info := range txHashes {
		tx := block.Transaction(info.TxHash)
		l.log.Debug(hex.EncodeToString(tx.Data()))

		parsedData, err := l.parsePayloadOnInput(hex.EncodeToString(tx.Data()), l.txMetaData.Header, l.txMetaData.Footer) //todo header
		if err != nil {
			return nil
		}

		result[tx.Hash().String()] = append(parsedData, info.Recipient)
	}

	return result
}

func (l *ListenData) parsePayloadOnInput(input string, header string, footer string) ([]string, error) { //move to  another class
	index := strings.Index(input, header)
	if index == -1 {
		return nil, errors.New("failed to found substring")
	}

	input = input[index:]
	index = strings.Index(input, footer)
	if index == -1 {
		return nil, errors.New("failed to found substring")
	}

	hexBytes, err := hex.DecodeString(input[:index])
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode string")
	}

	decodedString := string(hexBytes)

	return strings.Split(decodedString, ":"), nil
}

func (l *ListenData) GetTxHashes(logs []types.Log) []common.Hash {
	result := make([]common.Hash, 0)
	for _, event := range logs {
		result = append(result, event.TxHash)
	}

	return result
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
				Recipient:   payload[4],
				TxHash:      txHash,
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

	return nil, errors.New("data cant be converted to TX")
}

func (l *ListenData) parseRecipientFromEvent(events []types.Log) ([]RecipientInfo, error) {
	result := make([]RecipientInfo, 0)

	abiBytes, err := os.ReadFile("./internal/contract/erc20/erc20.abi") //todo config
	if err != nil {
		return nil, errors.Wrap(err, "failed to read file ")
	}

	contractAbi, err := abi.JSON(bytes.NewReader(abiBytes))
	if err != nil {
		return nil, errors.Wrap(err, "failed to  parse  abi ")
	}

	for _, vLog := range events {
		event, err := contractAbi.Unpack("DepositedERC20", vLog.Data)
		if err != nil {
			l.log.WithError(err).Error("failed to unpack abi")
			continue
		}
		if len(event) < 6 {
			l.log.Error("event too short")
			continue
		}

		result = append(result, RecipientInfo{
			Recipient: event[5].(string),
			TxHash:    vLog.TxHash,
		})
	}

	return result, nil
}
