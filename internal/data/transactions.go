package data

import (
	"gitlab.com/distributed_lab/kit/pgdb"
	"time"
)

type TransactionsQ interface {
	New() TransactionsQ
	Insert(data *Transactions) error
	Select() ([]Transactions, error)
	Page(pageParams pgdb.OffsetPageParams) TransactionsQ
	Update(data *Transactions) error
	FilterByReady() TransactionsQ
	FilterByNotReady() TransactionsQ
	FilterByNetworkFrom(networkFrom int32) TransactionsQ
	FilterByRecipient(address string) TransactionsQ
	FilterByPaymentID(paymentID string) TransactionsQ
}

type Transactions struct {
	PaymentID   string    `db:"payment_id" structs:"payment_id"`
	NetworkFrom int32     `db:"network_from" structs:"network_from"`
	TxHashFrom  string    `db:"tx_hash_from" structs:"tx_hash_from"`
	TxHashTo    string    `db:"tx_hash_to" structs:"tx_hash_to"`
	NetworkTo   int32     `db:"network_to" structs:"network_to"`
	Recipient   string    `db:"recipient" structs:"recipient"`
	Sender      string    `db:"sender" structs:"sender"`
	ValueTo     string    `db:"value_to" structs:"value_to"`
	Currency    string    `db:"currency" structs:"currency"`
	TimestampTo time.Time `db:"timestamp_to" structs:"timestamp_to"`
}
