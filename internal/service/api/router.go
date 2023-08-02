package api

import (
	"github.com/dl-only-tokens/back-listener/internal/data/pg"
	"github.com/dl-only-tokens/back-listener/internal/service/api/handlers"
	"github.com/go-chi/chi"
	"gitlab.com/distributed_lab/ape"
)

func (s *service) router() chi.Router {
	r := chi.NewRouter()

	r.Use(
		ape.RecoverMiddleware(s.log),
		ape.LoganMiddleware(s.log),
		ape.CtxMiddleware(
			handlers.CtxLog(s.log),
			handlers.CtxMasterQ(pg.NewMasterQ(s.cfg.DB())),
		),
	)
	r.Route("/integrations/back-listener", func(r chi.Router) {
		r.Route("/transactions", func(r chi.Router) {
			r.Get("/{address}", handlers.GetTxLists)
		})
	})

	return r
}
