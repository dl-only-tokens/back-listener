package handlers

import (
	"github.com/dl-only-tokens/back-listener/internal/data"
	"github.com/dl-only-tokens/back-listener/internal/service/api/requests"
	"github.com/dl-only-tokens/back-listener/resources"
	"gitlab.com/distributed_lab/ape"
	"gitlab.com/distributed_lab/ape/problems"
	"net/http"
)

func GetTxLists(w http.ResponseWriter, r *http.Request) {
	req, err := requests.NewGetTXsListRequest(r)
	if err != nil {
		Log(r).WithError(err).Error("failed to parse request")
		ape.RenderErr(w, problems.BadRequest(err)...)
		return
	}

	txs, err := MasterQ(r).TransactionsQ().New().FilterByAddress(req.Address).Select()
	if err != nil {
		Log(r).WithError(err).Error("failed to select txs by address")
		ape.RenderErr(w, problems.InternalError())
		return
	}
	if txs == nil {
		Log(r).WithError(err).Error("failed to empty select list")
		ape.RenderErr(w, problems.InternalError())
		return
	}

	ape.Render(w, prepareResponse(txs))
}

func prepareResponse(txs []data.Transactions) resources.GetTxListResponse {
	txBlobs := make([]resources.TxBlob, 0)
	for _, tx := range txs {
		blob := resources.TxBlob{
			Network:   tx.Network,
			PaymentId: tx.PaymentID,
			//Recipient: tx.Recipient,
			TxHash: tx.TxHash,
		}
		txBlobs = append(txBlobs, blob)
	}

	return resources.GetTxListResponse{
		Data: resources.GetTxList{
			Attributes: resources.GetTxListAttributes{
				Transactions: txBlobs,
			},
		},
	}

}
