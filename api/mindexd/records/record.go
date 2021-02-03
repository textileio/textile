package records

import "time"

type StorageDealRecord struct {
	ID                   string               `bson:"_id,omitempty"`
	PowName              string               `bson:"pow_name"`
	LastUpdatedAt        time.Time            `bson:"last_updated_at"`
	PowStorageDealRecord PowStorageDealRecord `bson:"pow_storage_deal_record"`
}

type RetrievalRecord struct {
	ID                 string             `bson:"_id,omitempty"`
	PowName            string             `bson:"pow_name"`
	LastUpdatedAt      time.Time          `bson:"last_updated_at"`
	PowRetrievalRecord PowRetrievalRecord `bson:"pow_retrieval_record"`
}

type PowStorageDealRecord struct {
	RootCid           string                       `bson:"root_cid"`
	Address           string                       `bson:"address"`
	Pending           bool                         `bson:"pending"`
	DealInfo          PowStorageDealRecordDealInfo `bson:"deal_info"`
	TransferSize      int64                        `bson:"transfer_size"`
	DataTransferStart int64                        `bson:"datatransfer_start"`
	DataTransferEnd   int64                        `bson:"datatransfer_end"`
	SealingStart      int64                        `bson:"sealing_start"`
	SealingEnd        int64                        `bson:"sealing_end"`
	ErrMsg            string                       `bson:"err_msg"`
	CreatedAt         int64                        `bson:"created_at"`
	UpdatedAt         int64                        `bson:"updated_at"`
}

type PowStorageDealRecordDealInfo struct {
	ProposalCid     string `bson:"proposal_cid"`
	StateId         uint64 `bson:"state_id"`
	StateName       string `bson:"state_name"`
	Miner           string `bson:"miner"`
	PieceCid        string `bson:"piece_cid"`
	Size            uint64 `bson:"size"`
	PricePerEpoch   uint64 `bson:"price_per_epoch"`
	StartEpoch      uint64 `bson:"start_epoch"`
	Duration        uint64 `bson:"duration"`
	DealId          uint64 `bson:"deal_id"`
	ActivationEpoch int64  `bson:"activation_epoch"`
	Message         string `bson:"message"`
}

type PowRetrievalRecord struct {
	ID                string                     `bson:"id"`
	Address           string                     `bson:"address"`
	DealInfo          PowRetrievalRecordDealInfo `bson:"deal_info"`
	DataTransferStart int64                      `bson:"datatransfer_start"`
	DataTransferEnd   int64                      `bson:"datatransfer_end"`
	ErrMsg            string                     `bson:"err_msg"`
	CreatedAt         int64                      `bson:"created_at"`
	UpdatedAt         int64                      `bson:"updated_at"`
}

type PowRetrievalRecordDealInfo struct {
	RootCid  string `bson:"root_cid"`
	Size     uint64 `bson:"size"`
	MinPrice uint64 `bson:"min_price"`
	Miner    string `bson:"miner"`
}
