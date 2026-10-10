package ann

import "encoding/binary"

// EncodeTypedAdjacency writes local GHA v1: 4-byte magic, version byte, 3 zero
// reserved bytes, uint32 layer count; each layer has uint16 degree, uint16
// zero reserved bits, and ascending big-endian positive int64 node IDs.
func EncodeTypedAdjacency(n TypedNode, c TypedConfig) ([]byte, error) {
	if err := ValidateTypedNode(n, c); err != nil {
		return nil, err
	}
	size := 12 + 4*len(n.Neighbors)
	for _, ids := range n.Neighbors {
		size += 8 * len(ids)
	}
	b := make([]byte, size)
	copy(b, "GHA\x00")
	b[4] = 1
	binary.BigEndian.PutUint32(b[8:12], uint32(len(n.Neighbors)))
	pos := 12
	for _, ids := range n.Neighbors {
		binary.BigEndian.PutUint16(b[pos:pos+2], uint16(len(ids)))
		pos += 4
		for _, id := range ids {
			binary.BigEndian.PutUint64(b[pos:pos+8], uint64(id))
			pos += 8
		}
	}
	return b, nil
}

// DecodeTypedAdjacency validates all counts, exact length and IDs before allocating
// adjacency storage. The caller verifies each referenced node and its layer.
func DecodeTypedAdjacency(b []byte, nodeID int64, level int, c TypedConfig) ([][]int64, error) {
	if c.Method != HNSW {
		return nil, ErrConfig
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	if nodeID < 1 || nodeID > c.MaxNodes || level < 0 || level > MaxLevel || len(b) < 12 || len(b) > 12+4*(MaxLevel+1)+8*(2*c.M+MaxLevel*c.M) || string(b[:4]) != "GHA\x00" || b[4] != 1 || b[5] != 0 || b[6] != 0 || b[7] != 0 || binary.BigEndian.Uint32(b[8:12]) != uint32(level+1) {
		return nil, ErrCorrupt
	}
	pos := 12
	for layer := 0; layer <= level; layer++ {
		if len(b)-pos < 4 {
			return nil, ErrCorrupt
		}
		count := int(binary.BigEndian.Uint16(b[pos : pos+2]))
		if count > layerCapacity(c.Config, layer) || b[pos+2] != 0 || b[pos+3] != 0 {
			return nil, ErrCorrupt
		}
		pos += 4
		if count > (len(b)-pos)/8 {
			return nil, ErrCorrupt
		}
		var prior uint64
		for i := 0; i < count; i++ {
			id := binary.BigEndian.Uint64(b[pos : pos+8])
			if id < 1 || id > uint64(c.MaxNodes) || id == uint64(nodeID) || id <= prior {
				return nil, ErrCorrupt
			}
			prior = id
			pos += 8
		}
	}
	if pos != len(b) {
		return nil, ErrCorrupt
	}
	neighbors := make([][]int64, level+1)
	pos = 12
	for layer := range neighbors {
		count := int(binary.BigEndian.Uint16(b[pos : pos+2]))
		pos += 4
		neighbors[layer] = make([]int64, count)
		for i := range neighbors[layer] {
			neighbors[layer][i] = int64(binary.BigEndian.Uint64(b[pos : pos+8]))
			pos += 8
		}
	}
	return neighbors, nil
}
