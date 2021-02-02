package records

type PowRetrievalRecord struct {
	Address           string
	DealInfo          PowRetrievalRecordDealInfo
	DataTransferStart int64
	DataTransferEnd   int64
	ErrMsg            string
	CreatedAt         int64
	UpdatedAt         int64
}

type PowRetrievalRecordDealInfo struct {
	RootCid  string
	Size     uint64
	MinPrice uint64
	Miner    string
}

type PowStorageDealRecord struct {
	RootCid           string
	Address           string
	Pending           bool
	DealInfo          PowStorageDealRecordDealInfo
	TransferSize      int64
	DataTransferStart int64
	DataTransferEnd   int64
	SealingStart      int64
	SealingEnd        int64
	ErrMsg            string
	CreatedAt         int64
	UpdatedAt         int64
}

type PowStorageDealRecordDealInfo struct {
	ProposalCid     string
	StateId         uint64
	StateName       string
	Miner           string
	PieceCid        string
	Size            uint64
	PricePerEpoch   uint64
	StartEpoch      uint64
	Duration        uint64
	DealId          uint64
	ActivationEpoch int64
	Message         string
}
