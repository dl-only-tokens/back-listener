package config

import (
	"github.com/pkg/errors"
	"gitlab.com/distributed_lab/figure"
	"gitlab.com/distributed_lab/kit/comfig"
	"gitlab.com/distributed_lab/kit/kv"
)

type MetaDataConfiger interface {
	MetaData() *MetaData
}

type MetaData struct {
	Header string `fig:"header"`
	Footer string `fig:"footer"`
}

func NewMetaDataConfiger(getter kv.Getter) MetaDataConfiger {
	return &metaDataConfiger{
		getter: getter,
	}
}

type metaDataConfiger struct {
	getter kv.Getter
	once   comfig.Once
}

func (c *metaDataConfiger) MetaData() *MetaData {
	return c.once.Do(func() interface{} {
		raw := kv.MustGetStringMap(c.getter, "meta_data")
		config := MetaData{}
		err := figure.Out(&config).From(raw).Please()
		if err != nil {
			panic(errors.Wrap(err, "failed to figure out"))
		}
		return &config
	}).(*MetaData)
}
