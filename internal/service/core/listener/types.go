package listener

import "github.com/ethereum/go-ethereum/common"

type TxInfo struct {
	Recipient   string
	PaymentID   string
	NetworkFrom string
	NetworkTo   string
}

type StateInfo struct {
	Name string
}

type RecipientInfo struct {
	Recipient string
	TxHash    common.Hash
}
