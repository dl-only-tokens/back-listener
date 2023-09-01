package listener

import "github.com/ethereum/go-ethereum/common"

type TxInfo struct {
	Recipient   string
	PaymentID   string
	NetworkFrom string
	NetworkTo   string
}

type StateInfo struct {
	Name      string
	LastBlock uint64
}

type RecipientInfo struct {
	Recipient string
	Sender    string
	TxHash    common.Hash
}
