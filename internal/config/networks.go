package config

import (
	figure "gitlab.com/distributed_lab/figure/v3"
	"gitlab.com/distributed_lab/kit/comfig"
	"gitlab.com/distributed_lab/kit/kv"
	"gitlab.com/distributed_lab/logan/v3/errors"
)

type NetworkConfiger interface {
	Network() *Network
}

type Network struct {
	NetInfoList []NetInfo `fig:"keys"`
}

type NetInfo struct {
	Name       string `fig:"name"`
	Address    string `fig:"address"`
	Rpc        string `fig:"rpc"`
	StartBlock uint64 `fiq:"start_block"`
	ChainID    int32  `fiq:"chain_id"`
}

func NewNetworkConfiger(getter kv.Getter) NetworkConfiger {
	return &networkconfiger{
		getter: getter,
	}
}

type networkconfiger struct {
	getter kv.Getter
	once   comfig.Once
}

func (c *networkconfiger) Network() *Network {
	return c.once.Do(func() interface{} {
		raw := kv.MustGetStringMap(c.getter, "networks")
		cfg := Network{}
		err := figure.Out(&cfg).With(figure.BaseHooks).From(raw).Please()
		if err != nil {
			panic(errors.Wrap(err, "failed to figure out"))
		}

		return &cfg
	}).(*Network)
}
