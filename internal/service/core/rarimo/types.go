package rarimo

type NetworkListResponse struct {
	Data     []Data        `json:"data"`
	Included []interface{} `json:"included"`
	Links    interface{}   `json:"links"`
}

type Data struct {
	Attributes    Attributes    `json:"attributes"`
	ID            string        `json:"id"`
	Relationships Relationships `json:"relationships"`
	Type          string        `json:"type"`
}

type Relationships struct {
	Tokens Tokens `json:"tokens"`
}

type Attributes struct {
	BridgeContract string      `json:"bridge_contract"`
	ChainParams    ChainParams `json:"chain_params"`
	ChainType      string      `json:"chain_type"`
	Icon           string      `json:"icon"`
	Name           string      `json:"name"`
}

type Tokens struct {
	Data []TokenData `json:"data"`
}

type TokenData struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

type ChainParams struct {
	ChainID      int64  `json:"chain_id"`
	ExplorerURL  string `json:"explorer_url"`
	NativeSymbol string `json:"native_symbol"`
}
