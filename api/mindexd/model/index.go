package model

import "time"

type MinerInfoSnapshot struct {
	CreatedAt time.Time `bson:"created_at"`
	MinerInfo MinerInfo `bson:"miner_info"`
}

type MinerInfo struct {
	MinerID   string       `bson:"_id"`
	Metadata  MetadataInfo `bson:"metadata"`
	Filecoin  FilecoinInfo `bson:"filecoin"`
	Textile   TextileInfo  `bson:"textile"`
	UpdatedAt time.Time    `bson:"updated_at"`
}

type MetadataInfo struct {
	Location  string    `bson:"location"`
	URL       string    `bson:"url"`
	Email     string    `bson:"email"`
	Twitter   string    `bson:"twitter"`
	MinerX    bool      `bson:"minerx"`
	UpdatedAt time.Time `bson:"updated_at"`
}

type FilecoinInfo struct {
	RelativePower    float64   `bson:"relative_power"`
	AskPrice         string    `bson:"ask_price"`
	AskVerifiedPrice string    `bson:"ask_verified_price"`
	MinPieceSize     int64     `bson:"min_piece_size"`
	MaxPieceSize     int64     `bson:"max_piece_size"`
	SectorSize       int64     `bson:"sector_size"`
	ActiveSectors    int64     `bson:"active_sectors"`
	FaultySectors    int64     `bson:"faulty_sectors"`
	UpdatedAt        time.Time `bson:"updated_at"`
}

type TextileInfo struct {
	Regions           map[string]TextileRegionInfo `bson:"regions"`
	DealsSummary      TextileDealsSummary          `bson:"deals_summary"`
	RetrievalsSummary TextileRetrievalSummary      `bson:"retrievals_summary"`
	UpdatedAt         time.Time                    `bson:"updated_at"`
}

type TextileDealsSummary struct {
	Total       int       `bson:"total"`
	Last        time.Time `bson:"last"`
	Failures    int       `bson:"failures"`
	LastFailure time.Time `bson:"last_failure"`
}

type TextileRetrievalSummary struct {
	Total       int       `bson:"total"`
	Last        time.Time `bson:"last"`
	Failures    int       `bson:"failures"`
	LastFailure time.Time `bson:"last_failure"`
}

type TextileRegionInfo struct {
	Deals      TextileDealsInfo      `bson:"deals"`
	Retrievals TextileRetrievalsInfo `bson:"retrievals"`
}

type TextileDealsInfo struct {
	Total int       `bson:"total"`
	Last  time.Time `bson:"last"`

	Failures    int       `bson:"failures"`
	LastFailure time.Time `bson:"last_failure"`

	TailTransfers []TransferMiBPerSec  `bson:"tail_transfers"`
	TailSealed    []SealedDurationMins `bson:"tail_sealed"`
}

type SealedDurationMins struct {
	SealedAt        time.Time `bson:"sealed_at"`
	DurationSeconds int       `bson:"duration_mins"`
}

type TransferMiBPerSec struct {
	TransferedAt time.Time `bson:"transfered_at"`
	MiBPerSec    float64   `bson:"mib_per_sec"`
}

type TextileRetrievalsInfo struct {
	Total         int                 `bson:"total"`
	Last          time.Time           `bson:"last"`
	Failures      int                 `bson:"failures"`
	LastFailure   time.Time           `bson:"last_failure"`
	TailTransfers []TransferMiBPerSec `bson:"tail_transfers"`
}
