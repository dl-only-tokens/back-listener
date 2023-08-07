package rarimo

import (
	"encoding/json"
	"github.com/dl-only-tokens/back-listener/internal/config"
	"gitlab.com/distributed_lab/logan/v3/errors"
	"net/http"
)

type RarimoHandler interface {
	GetContractsAddresses() ([]config.NetInfo, error)
}

type RarimoApi struct {
	api string
}

func NewRarimoHandler(api string) RarimoHandler {
	return RarimoApi{
		api: api,
	}
}

func (h RarimoApi) GetContractsAddresses() ([]config.NetInfo, error) {
	resp, err := http.Get(h.api)
	if err != nil {
		return nil, errors.Wrap(err, "failed to send request")
	}

	if resp.StatusCode >= 300 && resp.StatusCode < 500 {
		return nil, errors.New("bad response code")
	}

	decodedResponse := new(NetworkListResponse)

	if err := json.NewDecoder(resp.Body).Decode(&decodedResponse); err != nil {
		return nil, errors.New("failed to decode response ")
	}
	return h.parseResponse(decodedResponse.Data), nil
}

func (h RarimoApi) parseResponse(data []Data) []config.NetInfo {
	result := make([]config.NetInfo, 0)
	for _, net := range data {
		result = append(result, config.NetInfo{
			Name:    net.Attributes.Name,
			Address: net.Attributes.BridgeContract,
		})
	}

	return result
}
