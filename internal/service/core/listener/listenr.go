package listener

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"github.com/dl-only-tokens/back-listener/internal/config"
	"github.com/dl-only-tokens/back-listener/internal/data"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"gitlab.com/distributed_lab/logan/v3"
	"gitlab.com/distributed_lab/logan/v3/errors"
	"math/big"
	"os"
	"strings"
	"time"
)

type Listener interface {
	Run()
	GetNetwork() string
	Restart(parent context.Context)
}

type ListenData struct {
	chainID           int32
	chainName         string
	log               *logan.Entry
	pauseTime         int
	ctx               context.Context
	ctxCancelFunc     context.CancelFunc
	rpc               string
	address           string
	masterQ           data.MasterQ
	txMetaData        *config.MetaData
	healthCheckChan   chan Listener
	abiPath           string
	clientRPC         *ethclient.Client
	lastListenedBlock *big.Int
	lastBlock         uint64
}

type EthInfo struct {
	Address     string
	RPC         string
	ChainID     int32
	NetworkName string
}

func NewListener(parentCtx context.Context, log *logan.Entry, pauseTime int, ethInfo EthInfo, masterQ data.MasterQ, metaData *config.MetaData, healthCheckChan chan Listener, abiPath string, lastBlock *big.Int) Listener {
	ctx, cancelFunc := context.WithCancel(parentCtx)
	return &ListenData{
		chainID:           ethInfo.ChainID,
		chainName:         ethInfo.NetworkName,
		log:               log,
		pauseTime:         pauseTime,
		ctx:               ctx,
		ctxCancelFunc:     cancelFunc,
		rpc:               ethInfo.RPC,
		address:           ethInfo.Address,
		masterQ:           masterQ,
		txMetaData:        metaData,
		healthCheckChan:   healthCheckChan,
		abiPath:           abiPath,
		lastListenedBlock: lastBlock,
	}
}

func (l *ListenData) GetNetwork() string {
	return l.chainName
}

func (l *ListenData) Restart(parent context.Context) {
	l.ctx, l.ctxCancelFunc = context.WithCancel(parent)
	l.log.Debug("restart")
	l.Run()
}

func (l *ListenData) Run() {
	defer func() {
		l.healthCheckChan <- l

	}()
	var err error
	l.clientRPC, err = ethclient.Dial(l.rpc)
	if err != nil {
		l.log.WithError(err).Error("failed to failed to connect to node")
		return
	}

	l.run(l.ctx)

}

func (l *ListenData) run(ctx context.Context) {
	contractAddress := common.HexToAddress(l.address)
	var previewHash common.Hash
	var err error

	tickerTime := time.Duration(l.pauseTime) * time.Second
	ticker := time.NewTicker(1 * time.Second)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ticker.Reset(tickerTime)
			l.lastBlock, err = l.clientRPC.BlockNumber(ctx)
			if err != nil {
				l.log.WithError(err).Error(l.chainName, ": failed to get number of blocks")
				l.ctxCancelFunc()
				continue
			}

			if l.lastListenedBlock == nil {
				l.lastListenedBlock = big.NewInt(int64(l.lastBlock))
			}

			for l.lastBlock >= l.lastListenedBlock.Uint64() {
				block, err := l.clientRPC.BlockByNumber(context.Background(), l.lastListenedBlock)
				if err != nil {
					l.log.WithError(err).Error(l.chainName, ": failed to get last block ")
					l.ctxCancelFunc()
					continue
				}

				l.lastListenedBlock = l.lastListenedBlock.Add(l.lastListenedBlock, big.NewInt(1))

				hash := block.Hash()
				if previewHash == hash {
					continue
				}

				go l.indexContractTxs(block)

				query := ethereum.FilterQuery{
					BlockHash: &hash,
					Addresses: []common.Address{contractAddress},
				}
				previewHash = hash

				logs, err := l.clientRPC.FilterLogs(context.Background(), query)
				if err != nil {
					continue
				}

				suitableTXs, err := l.parseRecipientFromEvent(logs, block.Hash())
				if err != nil {
					l.log.WithError(err).Error("failed to  get suitable")
					continue
				}

				if len(suitableTXs) == 0 {
					continue
				}

				preapredTxs, err := l.prepareDataToInsert(l.getTxIntputsOnBlock(suitableTXs, block))
				if err != nil {
					l.log.WithError(err).Error("failed to  get suitable")
					continue
				}
				if err = l.insertTxs(preapredTxs); err != nil {
					l.log.WithError(err).Error("failed to use transaction")
					continue
				}
			}
		}
	}
}

func (l *ListenData) indexContractTxs(block *types.Block) {
	contractTXsOnBlock := l.filteringTx(block)
	if len(contractTXsOnBlock) == 0 {
		return
	}

	l.log.Debug("Indexer: contractTXsOnBlock:  ", contractTXsOnBlock)

	txs, err := l.prepareDataToInsert(contractTXsOnBlock)
	if err != nil {
		l.log.WithError(err).Error("failed to  get suitable")
		return
	}
	l.log.Debug("Indexer: txs:  ", txs)

	if err = l.insertTxs(txs); err != nil {
		l.log.Error(errors.Wrap(err, "failed to use transaction"))
		return
	}

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

func (l *ListenData) prepareDataToInsert(inputs map[string][]string) ([]data.Transactions, error) {
	response := make([]data.Transactions, 0)
	for txHash, payload := range inputs {
		networkTo, err := l.stringToInt32(payload[3])
		if err != nil {
			return nil, errors.Wrap(err, "failed to  prepare network  to")
		}
		networkFrom, err := l.stringToInt32(payload[2])
		if err != nil {
			return nil, errors.Wrap(err, "failed to  prepare network from")
		}

		tx := data.Transactions{
			PaymentID:   payload[1],
			NetworkFrom: networkFrom,
			NetworkTo:   networkTo,
			ValueTo:     payload[4],
		}

		if len(payload) > 5 {
			tx.Recipient = payload[5]
			tx.Sender = payload[6]
		}

		if tx.NetworkTo == l.chainID {
			tx.TxHashTo = txHash
			response = append(response, tx)
			continue
		}

		tx.TxHashFrom = txHash
		response = append(response, tx)
	}

	return response, nil
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

		finalTx, err := l.setTimestamp(l.packTX(selectedTxs[0], tx))
		if err != nil {
			return errors.Wrap(err, " failed to set value and timestamp ")
		}
		if err = l.masterQ.TransactionsQ().New().FilterByPaymentID(tx.PaymentID).Update(finalTx); err != nil {
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

func (l *ListenData) parseRecipientFromEvent(events []types.Log, blockHash common.Hash) ([]RecipientInfo, error) {
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
		if len(event) < receiverPositionOnEvent+1 {
			l.log.Error("event too short")
			continue
		}

		sender, err := l.getSender(vLog.TxHash.String(), blockHash, vLog.TxIndex)
		if err != nil {
			l.log.WithError(err).Error("failed to get sender")
			continue
		}

		result = append(result, RecipientInfo{
			Recipient: event[receiverPositionOnEvent].(string),
			TxHash:    vLog.TxHash,
			Sender:    sender,
		})
	}

	return result, nil
}

func (l *ListenData) filteringTx(block *types.Block) map[string][]string {
	result := make(map[string][]string)
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

func (l *ListenData) getSender(txHash string, blockHash common.Hash, txIndex uint) (string, error) {
	tx, _, err := l.clientRPC.TransactionByHash(l.ctx, common.HexToHash(txHash))
	if err != nil {
		return "", errors.Wrap(err, "failed to get transaction  by  hash")
	}

	sender, err := l.clientRPC.TransactionSender(l.ctx, tx, blockHash, txIndex)
	if err != nil {
		return "", errors.Wrap(err, "failed to get tx")
	}

	return sender.Hex(), nil
}

func (l *ListenData) setTimestamp(transaction *data.Transactions) (*data.Transactions, error) {
	tx, _, err := l.clientRPC.TransactionByHash(l.ctx, common.HexToHash(transaction.TxHashTo))
	if err != nil {
		return transaction, errors.Wrap(err, "failed to get transaction  by  hash")
	}

	transaction.TimestampTo = tx.Time()

	return transaction, nil
}

func (l *ListenData) stringToInt32(str string) (int32, error) {
	var n int32
	if _, err := fmt.Sscan(str, &n); err != nil {
		return -1, errors.Wrap(err, "failed to convert string to int")
	}

	return n, nil
}
