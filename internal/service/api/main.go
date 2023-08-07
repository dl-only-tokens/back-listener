package api

import (
	"github.com/dl-only-tokens/back-listener/internal/data/pg"
	"github.com/dl-only-tokens/back-listener/internal/service/core/handler"
	"gitlab.com/distributed_lab/logan/v3"
	"net"
	"net/http"

	"github.com/dl-only-tokens/back-listener/internal/config"
	"gitlab.com/distributed_lab/kit/copus/types"
	"gitlab.com/distributed_lab/logan/v3/errors"
)

type service struct {
	log      *logan.Entry
	copus    types.Copus
	listener net.Listener
	cfg      config.Config
}

func (s *service) run() error {
	s.log.Info("Service started")

	r := s.router()
	/////////////////
	listenHandler := handler.NewHandler(s.log, s.cfg.Network().NetInfoList, s.cfg.API(), pg.NewMasterQ(s.cfg.DB()))

	if err := listenHandler.Init(); err != nil {
		return errors.Wrap(err, "failed to init listeners")
	}
	go listenHandler.Run()
	///////////////// todo  make more clear
	if err := s.copus.RegisterChi(r); err != nil {
		return errors.Wrap(err, "cop failed")
	}

	return http.Serve(s.listener, r)
}

func newService(cfg config.Config) *service {
	return &service{
		log:      cfg.Log(),
		copus:    cfg.Copus(),
		listener: cfg.Listener(),
		cfg:      cfg,
	}
}

func Run(cfg config.Config) {
	if err := newService(cfg).run(); err != nil {
		panic(err)
	}
}
