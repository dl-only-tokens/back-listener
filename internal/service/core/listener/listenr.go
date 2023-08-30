package listener

import (
	"context"
	"encoding/hex"
	"fmt"
	"github.com/dl-only-tokens/back-listener/internal/config"
	"github.com/dl-only-tokens/back-listener/internal/data"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"gitlab.com/distributed_lab/logan/v3"
	"gitlab.com/distributed_lab/logan/v3/errors"
	"math/big"
	"strings"
	"time"
)

type Listener interface {
	Run()
	Restart(parent context.Context)
	IsIndexer() bool
}

type ListenData struct {
	chainID           int32
	chainName         string
	log               *logan.Entry
	pauseTime         int
	ctx               context.Context
	ctxCancelFunc     context.CancelFunc
	rpc               string
	masterQ           data.MasterQ
	txMetaData        *config.MetaData
	healthCheckChan   chan Listener
	abiPath           string
	clientRPC         *ethclient.Client
	lastListenedBlock *big.Int
	lastBlock         uint64
	isIndexer         bool
}

type EthInfo struct {
	RPC         string
	ChainID     int32
	NetworkName string
}

func NewListener(parentCtx context.Context, log *logan.Entry, pauseTime int, ethInfo EthInfo, masterQ data.MasterQ, metaData *config.MetaData, healthCheckChan chan Listener, abiPath string, lastBlock *big.Int, isIndexer bool) Listener {
	ctx, cancelFunc := context.WithCancel(parentCtx)
	return &ListenData{
		chainID:           ethInfo.ChainID,
		chainName:         ethInfo.NetworkName,
		log:               log,
		pauseTime:         pauseTime,
		ctx:               ctx,
		ctxCancelFunc:     cancelFunc,
		rpc:               ethInfo.RPC,
		masterQ:           masterQ,
		txMetaData:        metaData,
		healthCheckChan:   healthCheckChan,
		abiPath:           abiPath,
		lastListenedBlock: lastBlock,
		isIndexer:         isIndexer,
	}
}

func (l *ListenData) IsIndexer() bool {
	return l.isIndexer
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
	var previewHash common.Hash
	var err error

	tickerTime := time.Duration(l.pauseTime) * time.Second
	ticker := time.NewTicker(1 * time.Second)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ticker.Reset(1 * time.Second)
			if !l.isIndexer {
				ticker.Reset(tickerTime)
			}
			l.lastBlock, err = l.clientRPC.BlockNumber(ctx)
			if err != nil {
				l.log.WithError(err).Error(l.chainName, ": failed to get number of blocks")
				l.ctxCancelFunc()
				continue
			}

			if l.isIndexer {
				l.log.Debug("INDEXER ", l.chainName)
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

				previewHash = hash
				l.indexContractTxs(block)

			}
			if l.isIndexer {
				return
			}
		}
	}
}

func (l *ListenData) indexContractTxs(block *types.Block) {
	contractTXsOnBlock, err := l.filteringTx(block)
	if err != nil {
		l.log.WithError(err).Error("failed to filter tx")
		return
	}
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

	if err = l.insertTxs(txs, block.Time()); err != nil {
		l.log.WithError(err).Error("failed to use transaction")
		return
	}

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

		if len(payload) > 6 {
			tx.Currency = payload[5]
			tx.Recipient = payload[6]
			tx.Sender = payload[7]
		}

		if tx.NetworkTo == tx.NetworkFrom {
			tx.TxHashTo = txHash
			tx.TxHashFrom = txHash
			response = append(response, tx)
			continue
		}

		if tx.NetworkTo == l.chainID {
			tx.TxHashTo = txHash
			response = append(response, tx)
			continue
		}

		tx.TxHashFrom = txHash
		response = append(response, tx)
	}

	return l.txsToLowerCase(response), nil
}

func (l *ListenData) insertTxs(txs []data.Transactions, blockTime uint64) error {
	for _, tx := range txs {
		l.log.Debug(l.chainName, " tx: ", &tx)
		selectedTxs, err := l.masterQ.TransactionsQ().New().FilterByPaymentID(tx.PaymentID).Select()
		if err != nil {
			return errors.Wrap(err, "failed to  select tx to db")
		}

		if len(selectedTxs) == 0 {
			if tx.NetworkTo == l.chainID {
				tx.TimestampTo = time.Unix(int64(blockTime), 0)
			}

			if err = l.masterQ.TransactionsQ().New().Insert(&tx); err != nil {
				return errors.Wrap(err, "failed to  insert tx to db")
			}
			return nil
		}

		if len(selectedTxs[0].TxHashTo) != 0 && len(selectedTxs[0].TxHashFrom) != 0 {
			return nil
		}

		if tx.NetworkTo == l.chainID {
			tx.TimestampTo = time.Unix(int64(blockTime), 0)
		}
		if err = l.masterQ.TransactionsQ().New().FilterByPaymentID(tx.PaymentID).Update(l.packTX(tx, selectedTxs[0])); err != nil {
			return errors.Wrap(err, "failed to update tx to db")
		}

	}
	return nil
}

func (l *ListenData) filteringTx(block *types.Block) (map[string][]string, error) {
	result := make(map[string][]string)
	for _, evmTx := range block.Transactions() {
		if evmTx.To() != nil {
			parsedData, err := l.parsePayloadOnInput(hex.EncodeToString(evmTx.Data()), l.txMetaData.Header, l.txMetaData.Footer)
			if err != nil {
				continue
			}
			result[evmTx.Hash().String()] = parsedData
		}
	}

	return result, nil
}

func (l *ListenData) packTX(firstTX data.Transactions, secondTX data.Transactions) *data.Transactions {
	result := data.Transactions{}

	result.TxHashTo = secondTX.TxHashTo
	result.TimestampTo = secondTX.TimestampTo
	result.TxHashFrom = firstTX.TxHashFrom

	if firstTX.NetworkTo == l.chainID {
		result.TxHashTo = firstTX.TxHashTo
		result.TimestampTo = firstTX.TimestampTo
		result.TxHashFrom = secondTX.TxHashFrom
	}

	result.PaymentID = secondTX.PaymentID
	result.Currency = secondTX.Currency
	result.Sender = secondTX.Sender
	result.Recipient = secondTX.Recipient
	result.ValueTo = secondTX.ValueTo
	result.NetworkTo = secondTX.NetworkTo
	result.NetworkFrom = secondTX.NetworkFrom

	return &result
}

func (l *ListenData) txsToLowerCase(data []data.Transactions) []data.Transactions {
	for _, tx := range data {
		if len(tx.Sender) > 0 {
			strings.ToLower(tx.Sender)
		}
		if len(tx.Recipient) > 0 {
			strings.ToLower(tx.Recipient)
		}
		if len(tx.TxHashTo) > 0 {
			strings.ToLower(tx.TxHashTo)
		}
		if len(tx.TxHashFrom) > 0 {
			strings.ToLower(tx.TxHashFrom)
		}
	}
	return data
}

func (l *ListenData) stringToInt32(str string) (int32, error) {
	var n int32
	if _, err := fmt.Sscan(str, &n); err != nil {
		return -1, errors.Wrap(err, "failed to convert string to int")
	}

	return n, nil
}
