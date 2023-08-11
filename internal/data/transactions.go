package data

import "gitlab.com/distributed_lab/kit/pgdb"

type TransactionsQ interface {
	New() TransactionsQ
	Insert(data *Transactions) error
	FilterByAddress(address string) TransactionsQ
	Select() ([]Transactions, error)
	Page(pageParams pgdb.OffsetPageParams) TransactionsQ
}

type Transactions struct {
	PaymentID   string `db:"payment_id" structs:"payment_id"`
	NetworkFrom string `db:"network_from" structs:"network_from"`
	TxHash      string `db:"tx_hash" structs:"tx_hash"`
	NetworkTo   string `db:"network_to" structs:"network_to"`
	Recipient   string `db:"recipient" structs:"recipient"`
}
