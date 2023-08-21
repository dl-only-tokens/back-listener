/*
 * GENERATED. Do not modify. Your changes might be overwritten!
 */

package resources

type GetTxList struct {
	Key
	Attributes    GetTxListAttributes    `json:"attributes"`
	Relationships GetTxListRelationships `json:"relationships"`
}
type GetTxListResponse struct {
	Data     GetTxList `json:"data"`
	Included Included  `json:"included"`
}

type GetTxListListResponse struct {
	Data     []GetTxList `json:"data"`
	Included Included    `json:"included"`
	Links    *Links      `json:"links"`
}

// MustGetTxList - returns GetTxList from include collection.
// if entry with specified key does not exist - returns nil
// if entry with specified key exists but type or ID mismatches - panics
func (c *Included) MustGetTxList(key Key) *GetTxList {
	var getTxList GetTxList
	if c.tryFindEntry(key, &getTxList) {
		return &getTxList
	}
	return nil
}
