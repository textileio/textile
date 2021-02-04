package model

import "time"

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
	RelativePower float64   `bson:"relative_power"`
	AskPrice      int64     `bson:"ask_price"`
	MinPieceSize  int64     `bson:"min_piece_size"`
	MaxPieceSize  int64     `bson:"max_piece_size"`
	UpdatedAt     time.Time `bson:"updated_at"`
}

type TextileInfo struct {
	Deals      TextileDealsInfo      `bson:"deals"`
	Retrievals TextileRetrievalsInfo `bson:"retrievals"`
	UpdatedAt  time.Time             `bson:"updated_at"`
}

type TextileDealsInfo struct {
	Total int       `bson:"total"`
	Last  time.Time `bson:"last"`

	Failures    int       `bson:"failures"`
	LastFailure time.Time `bson:"last_failure"`

	LastTransferMiBPerSec     float64 `bson:"last_transfer_mib_per_sec"`
	Max7DaysTransferMiBPerSec float64 `bson:"max_7days_transfer_mib_per_sec"`

	LastDealSealedDurationMins float64 `bson:"last_deal_sealed_duration_mins"`
	MinDealSealedDurationMins  float64 `bson:"last_deal_sealed_duration_mins"`
}

type TextileRetrievalsInfo struct {
	Total                     int       `bson:"total"`
	Last                      time.Time `bson:"last"`
	Failures                  int       `bson:"failures"`
	LastFailure               time.Time `bson:"last_failure"`
	LastTransferMiBPerSec     float64   `bson:"last_Transfer_mib_per_sec"`
	Max7DaysTransferMiBPerSec float64   `bson:"max_7days_transfer_mib_per_sec"`
}
