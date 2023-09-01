package pg

import (
	"database/sql"
	"github.com/dl-only-tokens/back-listener/internal/data"
	"gitlab.com/distributed_lab/kit/pgdb"
)

type masterQ struct {
	db *pgdb.DB
}

func NewMasterQ(db *pgdb.DB) data.MasterQ {
	return &masterQ{
		db: db,
	}
}

func (q *masterQ) New() data.MasterQ {
	return NewMasterQ(q.db.Clone())
}

func (q *masterQ) TransactionsQ() data.TransactionsQ {
	return NewTransactionsQ(q.db)
}

func (q *masterQ) Transaction(fn func(data interface{}) error, data interface{}) error {
	return q.db.TransactionWithOptions(&sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
	}, func() error {
		return fn(data)
	})
}
