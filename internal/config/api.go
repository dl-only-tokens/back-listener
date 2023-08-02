package config

import (
	"github.com/pkg/errors"
	"gitlab.com/distributed_lab/figure"
	"gitlab.com/distributed_lab/kit/comfig"
	"gitlab.com/distributed_lab/kit/kv"
)

type APIConfiger interface {
	API() *API
}

type API struct {
	Endpoint string `fig:"endpoint"`
}

func NewAPIConfiger(getter kv.Getter) APIConfiger {
	return &apiconfiger{
		getter: getter,
	}
}

type apiconfiger struct {
	getter kv.Getter
	once   comfig.Once
}

func (c *apiconfiger) API() *API {
	return c.once.Do(func() interface{} {
		raw := kv.MustGetStringMap(c.getter, "api")
		config := API{}
		err := figure.Out(&config).From(raw).Please()
		if err != nil {
			panic(errors.Wrap(err, "failed to figure out"))
		}
		return &config
	}).(*API)
}
