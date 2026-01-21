package routing

import (
	"context"
	"crypto/sha256"
	"errors"
	"math"
	"math/big"
	"sort"
	"sync"
	"time"

	"github.com/nmxmxh/inos_v1/kernel/core/mesh/common"
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
	nodeID  string              // Our Node ID
	buckets [][]common.PeerInfo // 160 buckets for Kademlia routing

	// Local storage of values (ChunkHash -> Peers)
	store   sync.Map
	storeMu sync.RWMutex

	// Known peers lookup (ID -> PeerInfo)
	peers   map[string]common.PeerInfo
	peersMu sync.RWMutex

	alpha int // Concurrency parameter (default 3)
	k     int // Replication factor (default 20)

	transport common.Transport
	// Metrics
	metrics *DHTMetrics
}

func NewDHT(nodeID string, transport common.Transport, logger interface{}) *DHT { // Updated signature
	dht := &DHT{
		nodeID:    nodeID,
		buckets:   make([][]common.PeerInfo, 160),
		peers:     make(map[string]common.PeerInfo),
		alpha:     3,
		k:         20,
		transport: transport,
		metrics:   &DHTMetrics{},
	}

	for i := range dht.buckets {
		dht.buckets[i] = make([]common.PeerInfo, 0)
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

// GetEntryCount returns the number of unique chunks locally indexed.
func (d *DHT) GetEntryCount() uint32 {
	count := uint32(0)
	d.store.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	return count
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
	localCount := d.GetEntryCount()
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

// RemoveChunkPeer removes a peer from a chunk's advertisement list.
func (d *DHT) RemoveChunkPeer(chunkHash string, peerID string) error {
	d.storeMu.Lock()
	defer d.storeMu.Unlock()

	existing, exists := d.store.Load(chunkHash)
	if !exists {
		return nil
	}
	peerList := existing.([]string)
	updated := make([]string, 0, len(peerList))
	for _, p := range peerList {
		if p != peerID {
			updated = append(updated, p)
		}
	}
	if len(updated) == 0 {
		d.store.Delete(chunkHash)
		return nil
	}
	d.store.Store(chunkHash, updated)
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
func (d *DHT) FindNode(targetID string) []common.PeerInfo {
	d.peersMu.RLock()
	var allPeers []common.PeerInfo
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
func (d *DHT) AddPeer(peer common.PeerInfo) error {
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
	// 1. Normalize IDs to fixed 20-byte representation
	id1 := d.normalizeID(d.nodeID)
	id2 := d.normalizeID(targetID)

	// 2. XOR distance
	xor := new(big.Int).Xor(id1, id2)

	// 3. Bucket index = Count leading zeros in 160-bit space
	// If matching prefix is N bits, distance is in [2^(159-N), 2^(160-N))
	// Standard Kademlia: bucket index is log2(distance)
	bitLen := xor.BitLen()
	if bitLen == 0 {
		return 159 // Identical (usually doesn't happen for peers)
	}

	// We have 160 buckets. Bucket i covers distance [2^i, 2^(i+1))
	return bitLen - 1
}

func (d *DHT) normalizeID(id string) *big.Int {
	// If it's already a 20-byte raw string, use it.
	// Otherwise, hash it to ensure it fits in 160 bits.
	b := []byte(id)
	if len(b) != 20 {
		h := sha256.Sum256(b)
		b = h[:20]
	}
	return new(big.Int).SetBytes(b)
}

func (d *DHT) xorDistance(id1, id2 string) *big.Int {
	b1 := d.normalizeID(id1)
	b2 := d.normalizeID(id2)
	return new(big.Int).Xor(b1, b2)
}

// distance is an alias for xorDistance
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
		var candidates []common.PeerInfo
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
			go func(p common.PeerInfo) {
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
		go func(peer common.PeerInfo) {
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
func (d *DHT) iterativeFindNode(ctx context.Context, targetID string) ([]common.PeerInfo, error) {
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
		var candidates []common.PeerInfo
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
			go func(p common.PeerInfo) {
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

// peer index is built in LoadState and AddPeer

// GetPeer retrieves peer info if known
func (d *DHT) GetPeer(id string) (common.PeerInfo, bool) {
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
	SaveRoutingTable(buckets [][]common.PeerInfo) error
	SaveStore(store map[string][]string) error
	LoadRoutingTable() ([][]common.PeerInfo, error)
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
	d.peers = make(map[string]common.PeerInfo)

	// Rebuild peer index
	for _, bucket := range buckets {
		for _, peer := range bucket {
			d.peers[peer.ID] = peer
		}
	}

	return nil
}

// RemovePeer evicts a peer from routing tables and local cache.
func (d *DHT) RemovePeer(peerID string) {
	d.peersMu.Lock()
	delete(d.peers, peerID)
	for i := range d.buckets {
		bucket := d.buckets[i]
		next := bucket[:0]
		for _, peer := range bucket {
			if peer.ID != peerID {
				next = append(next, peer)
			}
		}
		d.buckets[i] = next
	}
	d.peersMu.Unlock()
}
