package data

type MasterQ interface {
	New() MasterQ
	TransactionsQ() TransactionsQ
	Transaction(func(data interface{}) error, interface{}) error
}
