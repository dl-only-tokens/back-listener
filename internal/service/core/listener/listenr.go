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
	"github.com/ethereum/go-ethereum/common/hexutil"
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
	chainID         string
	chainName       string
	log             *logan.Entry
	pauseTime       int
	ctx             context.Context
	rpc             string
	address         string
	masterQ         data.MasterQ
	txMetaData      *config.MetaData
	healthCheckChan chan StateInfo
	abiPath         string
}

type EthInfo struct {
	Address     string
	RPC         string
	ChainID     string
	NetworkName string
}

func NewListener(log *logan.Entry, pauseTime int, ethInfo EthInfo, masterQ data.MasterQ, metaData *config.MetaData, healthCheckChan chan StateInfo, abiPath string) Listener {
	return &ListenData{
		chainID:         ethInfo.ChainID,
		chainName:       ethInfo.NetworkName,
		log:             log,
		pauseTime:       pauseTime,
		ctx:             context.Background(),
		rpc:             ethInfo.RPC,
		address:         ethInfo.Address,
		masterQ:         masterQ,
		txMetaData:      metaData,
		healthCheckChan: healthCheckChan,
		abiPath:         abiPath,
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

	ticker := time.NewTicker(time.Duration(l.pauseTime) * time.Millisecond)

	go l.indexContractTxs(client)

	for {
		select {
		case <-l.ctx.Done():
			l.healthCheckChan <- StateInfo{
				Name: l.chainName,
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

			suitableTXs, err := l.parseRecipientFromEvent(logs, block.Hash(), client)
			if err != nil {
				l.log.WithError(err).Error("failed to  get suitable")
				continue
			}

			if len(suitableTXs) == 0 {
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

func (l *ListenData) GetNetwork() string {
	return l.chainName
}

func (l *ListenData) getTxIntputsOnBlock(txHashes []RecipientInfo, block *types.Block) map[string][]string {
	result := make(map[string][]string)
	for _, info := range txHashes {
		tx := block.Transaction(info.TxHash)

		parsedData, err := l.parsePayloadOnInput(hex.EncodeToString(tx.Data()), l.txMetaData.Header, l.txMetaData.Footer)
		if err != nil {
			return nil
		}

		result[tx.Hash().String()] = append(parsedData, info.Recipient, info.Sender)
	}

	return result
}

// parse some encoded info(payment id,  network  from  and  network  to ) from tx data
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

func (l *ListenData) prepareDataToInsert(inputs map[string][]string) []data.Transactions {
	response := make([]data.Transactions, 0)
	for txHash, payload := range inputs {

		tx := data.Transactions{
			PaymentID:   payload[1],
			NetworkFrom: payload[2],
			NetworkTo:   payload[3],
		}

		if len(payload) > 4 {
			tx.Recipient = payload[4]
			tx.Sender = payload[5]
		}

		if tx.NetworkTo == l.chainID {
			tx.TxHashTo = txHash
			response = append(response, tx)
			continue
		}

		tx.TxHashFrom = txHash
		response = append(response, tx)
	}

	return response
}

func (l *ListenData) insertTxs(any interface{}) error {
	txs, err := l.interfaceToTx(any)
	if err != nil {
		return errors.Wrap(err, "failed tom decode data")
	}
	for _, tx := range *txs {

		l.log.Debug(l.chainName, " tx: ", &tx)

		selectedTxs, err := l.masterQ.TransactionsQ().New().FilterByPaymentID(tx.PaymentID).Select()
		if err != nil {
			return errors.Wrap(err, "failed to  select tx to db")
		}

		if len(selectedTxs) == 0 {
			if err = l.masterQ.TransactionsQ().New().Insert(&tx); err != nil {
				return errors.Wrap(err, "failed to  insert tx to db")
			}
			return nil
		}

		if len(selectedTxs[0].TxHashTo) != 0 && len(selectedTxs[0].TxHashFrom) != 0 {
			return nil
		}

		if err = l.masterQ.TransactionsQ().New().FilterByPaymentID(tx.PaymentID).Update(l.packTX(selectedTxs[0], tx)); err != nil {
			return errors.Wrap(err, "failed to update tx to db")
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

func (l *ListenData) parseRecipientFromEvent(events []types.Log, blockHash common.Hash, client *ethclient.Client) ([]RecipientInfo, error) {
	result := make([]RecipientInfo, 0)

	abiBytes, err := os.ReadFile(l.abiPath)
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

		sender, err := l.getSender(vLog.TxHash.String(), client, blockHash, vLog.TxIndex)
		if err != nil {
			l.log.WithError(err).Error("failed to get sender")
			continue
		}

		result = append(result, RecipientInfo{
			Recipient: event[5].(string),
			TxHash:    vLog.TxHash,
			Sender:    sender,
		})
	}

	return result, nil
}

func (l *ListenData) indexContractTxs(client *ethclient.Client) {
	ticker := time.NewTicker(time.Duration(l.pauseTime) * time.Millisecond)
	var previewHash common.Hash

	for {
		select {
		case <-l.ctx.Done():
			l.healthCheckChan <- StateInfo{
				Name: l.chainName,
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

			previewHash = hash

			contractTXsOnBlock := l.filteringTx(block)
			if len(contractTXsOnBlock) == 0 {
				continue
			}

			l.log.Debug("Indexer: contractTXsOnBlock:  ", contractTXsOnBlock)

			txs := l.prepareDataToInsert(contractTXsOnBlock)
			l.log.Debug("Indexer: txs:  ", txs)

			if err = l.insertTxs(txs); err != nil {
				l.log.Error(errors.Wrap(err, "failed to use transaction"))
				return
			}

			break
		}

	}
}

func (l *ListenData) filteringTx(block *types.Block) map[string][]string {
	result := make(map[string][]string)
	if l.chainName == "BSC Testnet" {
		l.log.Debug(block.NumberU64(), "   ", l.chainName)
	}

	for _, tx := range block.Transactions() {
		if tx.To() != nil && bytes.Compare(tx.To().Bytes(), hexutil.MustDecode(l.address)) == 0 {

			parsedData, err := l.parsePayloadOnInput(hex.EncodeToString(tx.Data()), l.txMetaData.Header, l.txMetaData.Footer)
			if err != nil {
				continue
			}

			result[tx.Hash().String()] = append(parsedData)
		}

	}

	return result
}

func (l *ListenData) parseAddressesFromTXs(txs []data.Transactions) []string {
	result := make([]string, 0)
	for _, tx := range txs {
		result = append(result, tx.Recipient)
	}

	return result
}

func (l *ListenData) packTX(firstTX data.Transactions, secondTX data.Transactions) *data.Transactions {
	firstTX.TxHashTo = secondTX.TxHashTo
	return &firstTX
}

func (l *ListenData) getSender(txHash string, client *ethclient.Client, blockHash common.Hash, txIndex uint) (string, error) {
	tx, _, err := client.TransactionByHash(l.ctx, common.HexToHash(txHash))
	if err != nil {
		return "", errors.Wrap(err, "failed to get transaction  by  hash")
	}

	sender, err := client.TransactionSender(l.ctx, tx, blockHash, txIndex)
	if err != nil {
		return "", errors.Wrap(err, "failed to get tx")
	}
	return sender.Hex(), nil
}
