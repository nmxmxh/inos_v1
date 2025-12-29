package mesh

import (
	"context"
	"errors"
	"math"
	"math/big"
	"sort"
	"sync"
	"time"
)

// Transport interface is defined in types.go

// DHTMetrics tracks performance and state
type DHTMetrics struct {
	LookupLatencyP95  int64   `json:"lookup_latency_p95_ms"`
	SuccessRate       float64 `json:"success_rate"`
	BucketFillLevel   []int   `json:"bucket_fill_level"`
	LocalChunks       int64   `json:"local_chunks"`
	TotalQueries      int64   `json:"total_queries"`
	SuccessfulLookups int64   `json:"successful_lookups"`
	FailedQueries     int64   `json:"failed_queries"`
	storeMu           sync.RWMutex
}

// DHT implements a Kademlia-like distributed hash table.
type DHT struct {
	nodeID  string       // Our Node ID
	buckets [][]PeerInfo // 160 buckets for Kademlia routing

	// Local storage of values (ChunkHash -> Peers)
	store   sync.Map
	storeMu sync.RWMutex

	// Known peers lookup (ID -> PeerInfo)
	peers   map[string]PeerInfo
	peersMu sync.RWMutex

	alpha int // Concurrency parameter (default 3)
	k     int // Replication factor (default 20)

	transport Transport
	// Metrics
	metrics *DHTMetrics
}

func NewDHT(nodeID string, transport Transport, logger interface{}) *DHT { // Updated signature
	dht := &DHT{
		nodeID:    nodeID,
		buckets:   make([][]PeerInfo, 160),
		peers:     make(map[string]PeerInfo),
		alpha:     3,
		k:         20,
		transport: transport,
		metrics:   &DHTMetrics{},
	}

	for i := range dht.buckets {
		dht.buckets[i] = make([]PeerInfo, 0)
	}

	return dht
}

func (d *DHT) Start() error {
	// Start refresh loop?
	return nil
}

func (d *DHT) Stop() {
	// Stop loops
}

func (d *DHT) IsHealthy() bool {
	return true
}

func (d *DHT) GetHealthScore() float32 {
	return 1.0 // Simple placeholder
}

func (d *DHT) TotalPeers() uint32 {
	d.peersMu.RLock()
	defer d.peersMu.RUnlock()
	return uint32(len(d.peers))
}

func (d *DHT) GetState() interface{} {
	// Simplified state dump
	return map[string]interface{}{
		"peer_count": len(d.peers),
	}
}

func (d *DHT) GetTotalChunksCount() uint32 {
	networkSize := d.EstimateNetworkSize()
	avgChunksPerNode := d.getAverageChunksPerNode()

	return uint32(float64(networkSize) * avgChunksPerNode)
}

// EstimateNetworkSize estimates total peers in the network based on routing table density.
// Formula: N = 2^(160 - AvgPrefixLength) * TotalPeersInRoutingTable ?
// Better: N = (Total Peers / Distance Covered) * Total Space
// Implementation: User heuristic based on deepest bucket.
func (d *DHT) EstimateNetworkSize() int {
	d.peersMu.RLock()
	defer d.peersMu.RUnlock()

	if len(d.peers) < 2 {
		return 1
	}

	// Calculate average distance of K closest nodes
	// This gives us local density
	// We use the bucket index as a proxy for log-distance (160 - bucketIdx)

	totalWeight := 0.0
	count := 0

	for i, bucket := range d.buckets {
		if len(bucket) == 0 {
			continue
		}
		// Bucket i contains nodes with prefix matching i bits.
		// Distance is approx 2^(160-i).
		// Density contribution from this bucket: len(bucket) / 2^(160-i) ??
		// No, standard way:
		// Find the first bucket i that is NOT full?
		// Or simply: 2^(i) where i is the index of the deepest bucket with reasonable population.

		// Let's use the standard Kademlia estimation:
		// N = 2^(c) where c is the centroid of the prefix lengths.

		// Let's rely on the density of the closest-to-us bucket?
		// "The best estimate is derived from the deepest populated bucket."
		// Size ~= 2^i * len(bucket) / k ? No.

		// Let's stick effectively to: N = TotalKnown / FractionOfSpaceCovered
		// FractionOfSpaceCovered ~= 1 / 2^CP (CP = Common Prefix length with closest neighbor)
		// But this is noisy.

		// Simple & Robust:
		// Sum(2^-distance_exponent) for all peers?
		// Let's use the Centroid logic used in real DHTs (e.g. Mainline).
		// Estimate = (Known Peers) * 2^(160 - avg_prefix_len)
		// Wait, if I know 1 peer at distance 2^159 (bucket 0), and 1 at distance 2^0 (bucket 159).
		// The one in bucket 159 implies I explored deeply.

		// Let's use: Size = len(peers) * 2 ^ (Average Bucket Index) ??
		// No, deeper buckets (higher index) cover SMALLER space.
		// Bucket i covers 1/2^(i+1) of the space.
		// If we find P peers in bucket i, global density D = P / (TotalSpace / 2^(i+1))
		// Global N = D * TotalSpace = P * 2^(i+1)

		// We average this estimate across populated buckets.

		// Bucket index i: prefix match length i. (0 to 159)
		// Space fraction: 2^-(i+1)
		// Multiplier: 2^(i+1)
		for range bucket {
			// multiplier := math.Pow(2, float64(i+1))
			// We can't use math.Pow with 160.

			// We only care about relative magnitude.
			// Let's just return a simpler count for now if stats are low.
			// But user asked for "Estimator".

			// Let's use simple logic:
			// If we see nodes in bucket 10, it effectively means we explored 1/2^11 of the network.
			// If that bucket is full (20 nodes), then network is at least 20 * 2^11.

			weight := float64(len(bucket)) * float64(int(1)<<uint(i)) // Risk of overflow for standard ints?
			// i goes up to 159. 1<<159 overflows int64.
			// Use big.Int or log space.

			_ = weight
		}

		// New approach: Log space average
		// log2(N) = log2(P) + (i+1)
		// We average log2(N) estimates.

		if len(bucket) > 0 {
			estLogN := float64(i) + 1 + 0 // log2(len) is small correction
			// Actually just i is the dominant factor.
			totalWeight += estLogN
			count++
		}
	}

	if count == 0 {
		return len(d.peers)
	}

	avgLogN := totalWeight / float64(count)
	// If avgLogN is > 30, we return a cap because uint32 can't hold it.
	// But chunks count is uint32.

	if avgLogN > 31 {
		return math.MaxInt32
	}

	return int(math.Pow(2, avgLogN))
}

func (d *DHT) getAverageChunksPerNode() float64 {
	// Heuristic: Local store count
	localCount := d.getEntryCount()
	return float64(localCount) // Assume we are average
}

// Store advertises that a specific peer has a chunk.
func (d *DHT) Store(chunkHash string, peerID string, ttlSeconds int64) error {
	d.storeMu.Lock()
	defer d.storeMu.Unlock()

	// Update local knowledge first (Optimistic)
	existing, exists := d.store.Load(chunkHash)
	var peerList []string

	if exists {
		peerList = existing.([]string)
		// Dedup
		found := false
		for _, p := range peerList {
			if p == peerID {
				found = true
				break
			}
		}
		if !found {
			peerList = append(peerList, peerID)
		}
	} else {
		peerList = []string{peerID}
	}

	d.store.Store(chunkHash, peerList)

	// Replicate to K closest nodes to ensure persistence
	go d.replicateChunk(chunkHash, peerID)

	return nil
}

// FindPeers locates nodes that possess the given chunk.
func (d *DHT) FindPeers(chunkHash string) ([]string, error) {
	d.storeMu.RLock()
	// Check local cache first
	if peers, exists := d.store.Load(chunkHash); exists {
		d.storeMu.RUnlock()
		return peers.([]string), nil
	}
	d.storeMu.RUnlock()

	// Iterative Lookup in network
	return d.lookupChunk(chunkHash)
}

// FindNode returns the K closest nodes to a target ID.
func (d *DHT) FindNode(targetID string) []PeerInfo {
	d.peersMu.RLock()
	var allPeers []PeerInfo
	for _, p := range d.peers {
		allPeers = append(allPeers, p)
	}
	d.peersMu.RUnlock()

	// Sort by XOR distance
	sort.Slice(allPeers, func(i, j int) bool {
		distI := d.distance(allPeers[i].ID, targetID)
		distJ := d.distance(allPeers[j].ID, targetID)
		return distI.Cmp(distJ) < 0
	})

	if len(allPeers) > d.k {
		return allPeers[:d.k]
	}
	return allPeers
}

// AddPeer updates the routing table with a new peer.
func (d *DHT) AddPeer(peer PeerInfo) error {
	if peer.ID == d.nodeID {
		return nil
	}

	bucketIdx := d.getBucketIndex(peer.ID)

	d.peersMu.Lock()
	defer d.peersMu.Unlock()

	bucket := d.buckets[bucketIdx]

	// Check if already in bucket
	for i, p := range bucket {
		if p.ID == peer.ID {
			// Move to end (most recent)
			bucket = append(bucket[:i], bucket[i+1:]...)
			bucket = append(bucket, peer)
			d.buckets[bucketIdx] = bucket
			d.peers[peer.ID] = peer
			return nil
		}
	}

	// Not in bucket
	if len(bucket) < d.k {
		// Add to end
		bucket = append(bucket, peer)
		d.buckets[bucketIdx] = bucket
		d.peers[peer.ID] = peer
		return nil
	}

	// Bucket full - ping oldest node
	oldest := bucket[0]
	// We use a short timeout for the ping
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if d.transport != nil {
		if err := d.transport.Ping(ctx, oldest.ID); err != nil {
			// Oldest is dead - replace
			delete(d.peers, oldest.ID)
			bucket = bucket[1:]
			bucket = append(bucket, peer)
			d.buckets[bucketIdx] = bucket
			d.peers[peer.ID] = peer
			return nil
		}
	}

	// Oldest is alive - move to end
	bucket = bucket[1:]
	bucket = append(bucket, oldest)
	d.buckets[bucketIdx] = bucket

	// New peer is dropped (bucket stays full with alive nodes)
	return errors.New("bucket full, peer dropped")
}

// --- Internal Logic ---

func (d *DHT) getBucketIndex(targetID string) int {
	// XOR distance
	// Handle identical IDs
	if d.nodeID == targetID {
		return 159
	}

	xor := d.xorDistance(d.nodeID, targetID)
	// Bucket index = 159 - (BitLen - 1) ?
	// Kademlia: Distance d.
	// If d is in [2^i, 2^(i+1)), it goes in bucket i.
	// For 160 bit space, i goes from 0 to 159.
	//
	// Our logic:
	// First differing bit means log2(distance).
	// If matching prefix is N bits, distance is approx 2^(160-N).
	// So bucket index should relate to prefix length.

	// User suggested: 160 - bitLength
	bitLen := xor.BitLen()
	if bitLen == 0 {
		return 159
	}

	idx := 160 - bitLen
	if idx < 0 {
		return 0
	}
	if idx > 159 {
		return 159
	}
	return idx
}

// xorDistance helper without hashing
func (d *DHT) xorDistance(id1, id2 string) *big.Int {
	b1 := []byte(id1)
	b2 := []byte(id2)

	maxLen := len(b1)
	if len(b2) > maxLen {
		maxLen = len(b2)
	}

	padded1 := make([]byte, maxLen)
	padded2 := make([]byte, maxLen)

	// Right align? No, Kademlia ID usually big-endian hex or straight bytes.
	// Assuming raw string bytes for now or hex?
	// The user provided implementation assumes standard byte alignment.
	// We'll treat them as big-endian integers via SetBytes logic implicitly if we copy to end?
	// User's code: copy(padded[maxLen-len:], b) -> Right align.

	copy(padded1[maxLen-len(b1):], b1)
	copy(padded2[maxLen-len(b2):], b2)

	result := make([]byte, maxLen)
	for i := 0; i < maxLen; i++ {
		result[i] = padded1[i] ^ padded2[i]
	}

	return new(big.Int).SetBytes(result)
}

// Deprecated: old distance with re-hashing
func (d *DHT) distance(id1, id2 string) *big.Int {
	return d.xorDistance(id1, id2)
}

func (d *DHT) lookupChunk(chunkHash string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 1. Get k closest nodes from local routing table
	shortlist := d.FindNode(chunkHash)
	if len(shortlist) == 0 {
		return nil, errors.New("no peers in routing table")
	}

	// 2. Sort by distance
	sort.Slice(shortlist, func(i, j int) bool {
		distI := d.distance(shortlist[i].ID, chunkHash)
		distJ := d.distance(shortlist[j].ID, chunkHash)
		return distI.Cmp(distJ) < 0
	})

	// 3. Track visited and found
	visited := make(map[string]bool)
	visited[d.nodeID] = true

	var providers []string
	const maxRounds = 5

	for round := 0; round < maxRounds; round++ {
		// 4. Pick alpha unvisited nodes
		var candidates []PeerInfo
		for _, peer := range shortlist {
			if !visited[peer.ID] && len(candidates) < d.alpha {
				candidates = append(candidates, peer)
				visited[peer.ID] = true
			}
		}

		if len(candidates) == 0 {
			break
		}

		// 5. Parallel queries
		var mu sync.Mutex
		var wg sync.WaitGroup

		for _, peer := range candidates {
			wg.Add(1)
			go func(p PeerInfo) {
				defer wg.Done()

				if d.transport == nil {
					return
				}

				// Call transport
				values, closerPeers, err := d.transport.FindValue(ctx, p.ID, chunkHash)
				if err != nil {
					d.metrics.FailedQueries++
					return
				}

				mu.Lock()
				defer mu.Unlock()

				// Add providers if found
				if len(values) > 0 {
					providers = append(providers, values...)
				}

				// Update shortlist with closer peers
				for _, closer := range closerPeers {
					// Add if not already in shortlist
					found := false
					for _, existing := range shortlist {
						if existing.ID == closer.ID {
							found = true
							break
						}
					}
					if !found {
						shortlist = append(shortlist, closer)
					}
				}
			}(peer)
		}

		wg.Wait()

		// 6. Check if we found enough providers
		if len(providers) >= d.k {
			break
		}

		// 7. Re-sort shortlist for next round
		sort.Slice(shortlist, func(i, j int) bool {
			distI := d.distance(shortlist[i].ID, chunkHash)
			distJ := d.distance(shortlist[j].ID, chunkHash)
			return distI.Cmp(distJ) < 0
		})

		// Keep only k closest
		if len(shortlist) > d.k {
			shortlist = shortlist[:d.k]
		}
	}

	d.metrics.storeMu.Lock()
	d.metrics.TotalQueries++
	if len(providers) > 0 {
		d.metrics.SuccessfulLookups++
	} else {
		d.metrics.FailedQueries++ // Or just not found
	}
	d.metrics.storeMu.Unlock()

	if len(providers) == 0 {
		return nil, errors.New("chunk not found")
	}

	return providers, nil
}

func (d *DHT) replicateChunk(chunkHash, peerID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 1. Find K closest neighbors to chunkHash (Iterative Node Lookup)
	closestPeers, err := d.iterativeFindNode(ctx, chunkHash)
	if err != nil {
		// Log error?
		return
	}

	// 2. Send STORE(chunkHash, peerID) to them
	var wg sync.WaitGroup
	for _, p := range closestPeers {
		wg.Add(1)
		go func(peer PeerInfo) {
			defer wg.Done()
			if d.transport != nil {
				// We store the provider ID as the value
				_ = d.transport.Store(ctx, peer.ID, chunkHash, []byte(peerID))
			}
		}(p)
	}
	wg.Wait()
}

// iterativeFindNode performs the Kademlia Node Lookup to find the K closest nodes to a target
func (d *DHT) iterativeFindNode(ctx context.Context, targetID string) ([]PeerInfo, error) {
	// 1. Start with alpha closest nodes from local routing table
	shortlist := d.FindNode(targetID)
	if len(shortlist) == 0 {
		return nil, errors.New("no peers in routing table")
	}

	// 2. Sort by distance
	sort.Slice(shortlist, func(i, j int) bool {
		distI := d.distance(shortlist[i].ID, targetID)
		distJ := d.distance(shortlist[j].ID, targetID)
		return distI.Cmp(distJ) < 0
	})

	visited := make(map[string]bool)
	visited[d.nodeID] = true

	// Keep track of the closest node found so far to detect convergence
	// closestNode := shortlist[0]

	const maxRounds = 5
	for round := 0; round < maxRounds; round++ {
		// Pick alpha unvisited
		var candidates []PeerInfo
		for _, peer := range shortlist {
			if !visited[peer.ID] && len(candidates) < d.alpha {
				candidates = append(candidates, peer)
				visited[peer.ID] = true
			}
		}

		if len(candidates) == 0 {
			break
		}

		var mu sync.Mutex
		var wg sync.WaitGroup

		for _, peer := range candidates {
			wg.Add(1)
			go func(p PeerInfo) {
				defer wg.Done()
				if d.transport == nil {
					return
				}

				// FIND_NODE RPC
				nodes, err := d.transport.FindNode(ctx, p.ID, targetID)
				if err != nil {
					return
				}

				mu.Lock()
				defer mu.Unlock()

				for _, n := range nodes {
					// Add if new
					found := false
					for _, existing := range shortlist {
						if existing.ID == n.ID {
							found = true
							break
						}
					}
					if !found && n.ID != d.nodeID {
						shortlist = append(shortlist, n)
					}
				}
			}(peer)
		}
		wg.Wait()

		// Sort and trim
		sort.Slice(shortlist, func(i, j int) bool {
			distI := d.distance(shortlist[i].ID, targetID)
			distJ := d.distance(shortlist[j].ID, targetID)
			return distI.Cmp(distJ) < 0
		})

		if len(shortlist) > d.k {
			shortlist = shortlist[:d.k]
		}
	}

	return shortlist, nil
}

func (d *DHT) Refresh() {
	// Refresh buckets that haven't been touched in T_refresh
	// For V1: Iterate buckets, if LastContact > 1 hour, pick random ID in range and iterativeFindNode
}

func (d *DHT) getEntryCount() uint32 {
	count := 0
	d.store.Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	return uint32(count)
}

// GetPeer retrieves peer info if known
func (d *DHT) GetPeer(id string) (PeerInfo, bool) {
	d.peersMu.RLock()
	defer d.peersMu.RUnlock()
	p, ok := d.peers[id]
	return p, ok
}

func (d *DHT) GetMetrics() *DHTMetrics {
	d.metrics.storeMu.RLock()
	defer d.metrics.storeMu.RUnlock()

	// Calculate bucket fill levels
	bucketLevels := make([]int, len(d.buckets))
	for i, bucket := range d.buckets {
		bucketLevels[i] = len(bucket)
	}
	d.metrics.BucketFillLevel = bucketLevels

	// Calculate success rate
	total := d.metrics.TotalQueries
	if total > 0 {
		d.metrics.SuccessRate = float64(d.metrics.SuccessfulLookups) / float64(total)
	}

	return d.metrics
}

// Persistence interface
type DHTStore interface {
	SaveRoutingTable(buckets [][]PeerInfo) error
	SaveStore(store map[string][]string) error
	LoadRoutingTable() ([][]PeerInfo, error)
	LoadStore() (map[string][]string, error)
}

func (d *DHT) SaveState(store DHTStore) error {
	// Convert sync.Map to regular map
	storeMap := make(map[string][]string)
	d.store.Range(func(key, value interface{}) bool {
		storeMap[key.(string)] = value.([]string)
		return true
	})

	if err := store.SaveStore(storeMap); err != nil {
		return err
	}

	return store.SaveRoutingTable(d.buckets)
}

func (d *DHT) LoadState(store DHTStore) error {
	storeMap, err := store.LoadStore()
	if err != nil {
		return err
	}

	buckets, err := store.LoadRoutingTable()
	if err != nil {
		return err
	}

	// Restore store
	for key, value := range storeMap {
		d.store.Store(key, value)
	}

	// Restore routing table
	d.peersMu.Lock()
	defer d.peersMu.Unlock()

	d.buckets = buckets
	d.peers = make(map[string]PeerInfo)

	// Rebuild peer index
	for _, bucket := range buckets {
		for _, peer := range bucket {
			d.peers[peer.ID] = peer
		}
	}

	return nil
}
