package config

import (
	"gitlab.com/distributed_lab/kit/comfig"
	"gitlab.com/distributed_lab/kit/copus"
	"gitlab.com/distributed_lab/kit/copus/types"
	"gitlab.com/distributed_lab/kit/kv"
	"gitlab.com/distributed_lab/kit/pgdb"
)

type Config interface {
	comfig.Logger
	pgdb.Databaser
	types.Copuser
	NetworkConfiger
	MetaDataConfiger

	ChainListenerConfiger
	comfig.Listenerer
}

type config struct {
	comfig.Logger
	pgdb.Databaser
	types.Copuser
	comfig.Listenerer
	NetworkConfiger
	MetaDataConfiger
	ChainListenerConfiger
	getter kv.Getter
}

func New(getter kv.Getter) Config {
	return &config{
		getter:                getter,
		MetaDataConfiger:      NewMetaDataConfiger(getter),
		NetworkConfiger:       NewNetworkConfiger(getter),
		Databaser:             pgdb.NewDatabaser(getter),
		Copuser:               copus.NewCopuser(getter),
		Listenerer:            comfig.NewListenerer(getter),
		ChainListenerConfiger: NewChainListenerConfiger(getter),
		Logger:                comfig.NewLogger(getter, comfig.LoggerOpts{}),
	}
}
