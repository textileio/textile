package local

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ipfs/go-cid"
	pb "github.com/textileio/textile/api/buckets/pb"
)

// ArchiveStatusTimeout is the timeout used when requesting a single status message.
var ArchiveStatusTimeout = time.Second * 5

// ArchiveRemote requests an archive of the current remote bucket.
func (b *Bucket) ArchiveRemote(ctx context.Context) error {
	b.Lock()
	defer b.Unlock()
	ctx, err := b.context(ctx)
	if err != nil {
		return err
	}
	if _, err := b.clients.Buckets.Archive(ctx, b.Key()); err != nil {
		return err
	}
	return nil
}

// ArchiveStatusMessage is used to wrap an archive status message.
type ArchiveStatusMessage struct {
	Type            ArchiveMessageType
	Message         string
	Error           error
	InactivityClose bool
}

// ArchiveMessageType is the type of status message.
type ArchiveMessageType int

const (
	// ArchiveMessage accompanies an informational message.
	ArchiveMessage ArchiveMessageType = iota
	// ArchiveWarning accompanies a warning state.
	ArchiveWarning
	// ArchiveError accompanies an error state.
	ArchiveError
	// ArchiveSuccess accompanies a successful state.
	ArchiveSuccess
)

// ArchiveStatus returns the current archive status.
// When watch is true, the channel remains open, delivering all messages.
func (b *Bucket) ArchiveStatus(ctx context.Context, watch bool) (<-chan ArchiveStatusMessage, error) {
	b.Lock()
	defer b.Unlock()
	ctx, err := b.context(ctx)
	if err != nil {
		return nil, err
	}
	key := b.Key()
	rep, err := b.clients.Buckets.ArchiveStatus(ctx, key)
	if err != nil {
		return nil, err
	}
	msgs := make(chan ArchiveStatusMessage)
	go func() {
		defer close(msgs)
		switch rep.GetStatus() {
		case pb.ArchiveStatusResponse_STATUS_FAILED:
			msgs <- ArchiveStatusMessage{
				Type:    ArchiveWarning,
				Message: "Archive failed with message: " + rep.GetFailedMsg(),
			}
		case pb.ArchiveStatusResponse_STATUS_CANCELED:
			msgs <- ArchiveStatusMessage{
				Type:    ArchiveWarning,
				Message: "Archive was superseded by a new executing archive",
			}
		case pb.ArchiveStatusResponse_STATUS_EXECUTING:
			msgs <- ArchiveStatusMessage{
				Type:    ArchiveMessage,
				Message: "Archive is currently executing, grab a coffee and be patient...",
			}
		case pb.ArchiveStatusResponse_STATUS_DONE:
			msgs <- ArchiveStatusMessage{
				Type:    ArchiveSuccess,
				Message: "Archive executed successfully!",
			}
		default:
			msgs <- ArchiveStatusMessage{
				Type:    ArchiveWarning,
				Message: "Archive status unknown",
			}
		}
		if watch {
			ch := make(chan string)
			wCtx, cancel := context.WithCancel(ctx)
			defer cancel()
			var err error
			go func() {
				err = b.clients.Buckets.ArchiveWatch(wCtx, key, ch)
				close(ch)
			}()
			for msg := range ch {
				msgs <- ArchiveStatusMessage{Type: ArchiveMessage, Message: "\t " + msg}
			}
			if err != nil {
				if strings.Contains(err.Error(), "RST_STREAM") {
					msgs <- ArchiveStatusMessage{Type: ArchiveError, InactivityClose: true}
					return
				}
				msgs <- ArchiveStatusMessage{Type: ArchiveError, Error: err}
			}
		}
	}()
	return msgs, nil
}

func isJobStatusFinal(status pb.ArchiveStatusResponse_Status) (bool, error) {
	switch status {
	case pb.ArchiveStatusResponse_STATUS_FAILED, pb.ArchiveStatusResponse_STATUS_CANCELED, pb.ArchiveStatusResponse_STATUS_DONE:
		return true, nil
	case pb.ArchiveStatusResponse_STATUS_EXECUTING:
		return false, nil
	}
	return true, fmt.Errorf("unknown job status")

}

// ArchiveInfo wraps info about an archive.
type ArchiveInfo struct {
	Key     string  `json:"key"`
	Archive Archive `json:"archive"`
}

// Archive describes the state of an archive.
type Archive struct {
	Cid   cid.Cid       `json:"cid"`
	Deals []ArchiveDeal `json:"deals"`
}

// ArchiveDeal describes an archive deal.
type ArchiveDeal struct {
	ProposalCid cid.Cid `json:"proposal_cid"`
	Miner       string  `json:"miner"`
}

// ArchiveInfo returns information about the current archvie.
func (b *Bucket) ArchiveInfo(ctx context.Context) (info ArchiveInfo, err error) {
	b.Lock()
	defer b.Unlock()
	ctx, err = b.context(ctx)
	if err != nil {
		return
	}
	rep, err := b.clients.Buckets.ArchiveInfo(ctx, b.Key())
	if err != nil {
		return
	}
	return pbArchiveInfoToArchiveInfo(rep)
}

func pbArchiveInfoToArchiveInfo(pi *pb.ArchiveInfoResponse) (info ArchiveInfo, err error) {
	info.Key = pi.Key
	if pi.Archive != nil {
		info.Archive.Cid, err = cid.Decode(pi.Archive.Cid)
		if err != nil {
			return
		}
		deals := make([]ArchiveDeal, len(pi.Archive.Deals))
		for i, d := range pi.Archive.Deals {
			deals[i].Miner = d.Miner
			deals[i].ProposalCid, err = cid.Decode(d.ProposalCid)
			if err != nil {
				return
			}
		}
	}
	return info, err
}
