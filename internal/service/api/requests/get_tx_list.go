package requests

import (
	"github.com/go-chi/chi"
	"github.com/pkg/errors"
	"gitlab.com/distributed_lab/urlval"
	"net/http"
	"strings"
)

const (
	AddressPathParam = "address"
)

type GetTXsListRequest struct {
	Address string `url:"-"`
}

func NewGetTXsListRequest(r *http.Request) (GetTXsListRequest, error) {
	request := GetTXsListRequest{}
	if err := urlval.Decode(r.URL.Query(), &request); err != nil {
		return request, errors.Wrap(err, "failed to decode query")
	}
	request.Address = strings.ToLower(chi.URLParam(r, AddressPathParam))

	return request, nil
}
