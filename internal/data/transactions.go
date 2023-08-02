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
	ID        int64  `db:"id" struct:"-"`
	PaymentID string `db:"payment_id" structs:"payment_id"`
	Recipient string `db:"recipient" structs:"recipient"`
	TxHash    string `db:"tx_hash" structs:"tx_hash"`
	Network   string `db:"network" structs:"network"`
}
