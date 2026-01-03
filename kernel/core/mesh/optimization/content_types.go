package optimization

// ContentMerkleTree represents a merkle tree for content verification and delta replication
// This is separate from the gossip MerkleTree which is used for message reconciliation
type ContentMerkleTree struct {
	Root   string              `json:"root"`
	Leaves []ContentMerkleLeaf `json:"leaves"`
	Depth  int                 `json:"depth"`
}

// ContentMerkleLeaf represents a leaf node in the content merkle tree
type ContentMerkleLeaf struct {
	Index int    `json:"index"`
	Hash  string `json:"hash"`
	Data  []byte `json:"data"`
}
