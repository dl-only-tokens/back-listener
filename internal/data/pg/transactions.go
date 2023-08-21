package pg

import (
	"fmt"
	sq "github.com/Masterminds/squirrel"
	"github.com/dl-only-tokens/back-listener/internal/data"
	"github.com/fatih/structs"
	"gitlab.com/distributed_lab/kit/pgdb"
	"gitlab.com/distributed_lab/logan/v3/errors"
)

const transactionTableName = "transactions"

const (
	idField      = "id"
	addressField = "recipient"
	paymentID    = "payment_id"
)

func NewTransactionsQ(db *pgdb.DB) data.TransactionsQ {
	return &TransactionsQ{
		db:  db,
		sql: sq.Select("b.*").From(fmt.Sprintf("%s as b", transactionTableName)),
		upd: sq.Update(transactionTableName),
	}
}

type TransactionsQ struct {
	db  *pgdb.DB
	sql sq.SelectBuilder
	upd sq.UpdateBuilder
}

func (q *TransactionsQ) New() data.TransactionsQ {
	return NewTransactionsQ(q.db.Clone())
}

func (q *TransactionsQ) Update(client *data.Transactions) error {
	clauses := structs.Map(client)
	if err := q.db.Exec(q.upd.SetMap(clauses)); err != nil {
		return errors.Wrap(err, "failed to update client")
	}

	return nil
}

func (q *TransactionsQ) Select() ([]data.Transactions, error) {
	var result []data.Transactions
	err := q.db.Select(&result, q.sql)
	if err != nil {
		return nil, errors.Wrap(err, "failed to select txs")
	}

	return result, nil
}

func (q *TransactionsQ) Insert(value *data.Transactions) error {
	clauses := structs.Map(value)

	if err := q.db.Exec(sq.Insert(transactionTableName).SetMap(clauses)); err != nil {
		return errors.Wrap(err, "failed to insert tx")
	}

	return nil
}

func (q *TransactionsQ) FilterByAddress(address string) data.TransactionsQ {
	q.sql = q.sql.Where(sq.Eq{addressField: address})
	q.upd = q.upd.Where(sq.Eq{addressField: address})

	return q
}

func (q *TransactionsQ) FilterByPaymentID(paymentID string) data.TransactionsQ {
	q.sql = q.sql.Where(sq.Eq{paymentID: paymentID})
	q.upd = q.upd.Where(sq.Eq{paymentID: paymentID})

	return q
}

func (q *TransactionsQ) Page(pageParams pgdb.OffsetPageParams) data.TransactionsQ {
	q.sql = pageParams.ApplyTo(q.sql, idField)

	return q
}
