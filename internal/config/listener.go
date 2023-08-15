package config

import (
	"github.com/pkg/errors"
	"gitlab.com/distributed_lab/figure"
	"gitlab.com/distributed_lab/kit/comfig"
	"gitlab.com/distributed_lab/kit/kv"
)

type ChainListenerConfiger interface {
	ChainListener() *ChainListener
}

type ChainListener struct {
	PauseTime int    `fig:"pause_time"`
	AbiPath   string `fig:"abi_path"`
}

func NewChainListenerConfiger(getter kv.Getter) ChainListenerConfiger {
	return &chainListenerConfiger{
		getter: getter,
	}
}

type chainListenerConfiger struct {
	getter kv.Getter
	once   comfig.Once
}

func (c *chainListenerConfiger) ChainListener() *ChainListener {
	return c.once.Do(func() interface{} {
		raw := kv.MustGetStringMap(c.getter, "chain_listener")
		config := ChainListener{}
		err := figure.Out(&config).From(raw).Please()
		if err != nil {
			panic(errors.Wrap(err, "failed to figure out"))
		}
		return &config
	}).(*ChainListener)
}
