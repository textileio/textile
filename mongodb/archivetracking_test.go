package mongodb_test

import (
	"context"
	"crypto/rand"
	"testing"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-threads/core/thread"
	. "github.com/textileio/textile/v2/mongodb"
)

func TestArchiveTracking_Create(t *testing.T) {
	db := newDB(t)
	col, err := NewArchiveTracking(context.Background(), db)
	require.NoError(t, err)

	ctx := context.Background()
	dbID := thread.NewIDV1(thread.Raw, 16)
	dbToken := thread.Token("token")
	bucketKey := "buckKey"
	jid := "jobID1"
	bucketRoot, _ := cid.Decode("QmSnuWmxptJZdLJpKRarxBMS2Ju2oANVrgbr2xWbie9b2D")
	_, key, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	err = col.CreateArchive(ctx, dbID, dbToken, bucketKey, jid, bucketRoot, thread.NewLibp2pPubKey(key))
	require.NoError(t, err)
}

func TestArchiveTracking_Get(t *testing.T) {
	db := newDB(t)
	col, err := NewArchiveTracking(context.Background(), db)
	require.NoError(t, err)
	ctx := context.Background()

	dbID := thread.NewIDV1(thread.Raw, 16)
	dbToken := thread.Token("token")
	bucketKey := "buckKey"
	jid := "jobID1"
	bucketRoot, _ := cid.Decode("QmSnuWmxptJZdLJpKRarxBMS2Ju2oANVrgbr2xWbie9b2D")
	_, key, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	err = col.CreateArchive(ctx, dbID, dbToken, bucketKey, jid, bucketRoot, thread.NewLibp2pPubKey(key))
	require.NoError(t, err)

	ta, err := col.Get(ctx, jid)
	require.NoError(t, err)
	require.Equal(t, jid, ta.JID)
	require.Equal(t, dbID, ta.DbID)
	require.Equal(t, dbToken, ta.DbToken)
	require.Equal(t, bucketKey, ta.BucketKey)
	require.Equal(t, bucketRoot, ta.BucketRoot)
	require.Equal(t, thread.NewLibp2pPubKey(key), ta.Owner)
	require.True(t, time.Since(ta.ReadyAt) > 0)
	require.True(t, ta.Active)
}

func TestArchiveTracking_GetReadyToCheck(t *testing.T) {
	db := newDB(t)
	col, err := NewArchiveTracking(context.Background(), db)
	require.NoError(t, err)
	ctx := context.Background()

	tas, err := col.GetReadyToCheck(ctx, 10)
	require.NoError(t, err)
	require.Equal(t, 0, len(tas))

	dbID := thread.NewIDV1(thread.Raw, 16)
	dbToken := thread.Token("token")
	bucketKey := "buckKey"
	jid := "jobID1"
	bucketRoot, _ := cid.Decode("QmSnuWmxptJZdLJpKRarxBMS2Ju2oANVrgbr2xWbie9b2D")
	_, key, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	err = col.CreateArchive(ctx, dbID, dbToken, bucketKey, jid, bucketRoot, thread.NewLibp2pPubKey(key))
	require.NoError(t, err)

	tas, err = col.GetReadyToCheck(ctx, 10)
	require.NoError(t, err)
	require.Equal(t, 1, len(tas))
	require.Equal(t, jid, tas[0].JID)
	require.Equal(t, dbID, tas[0].DbID)
	require.Equal(t, dbToken, tas[0].DbToken)
	require.Equal(t, bucketKey, tas[0].BucketKey)
	require.Equal(t, bucketRoot, tas[0].BucketRoot)
	require.Equal(t, thread.NewLibp2pPubKey(key), tas[0].Owner)
	require.True(t, time.Since(tas[0].ReadyAt) > 0)
	require.True(t, tas[0].Active)
}

func TestArchiveTracking_Finalize(t *testing.T) {
	db := newDB(t)
	col, err := NewArchiveTracking(context.Background(), db)
	require.NoError(t, err)
	ctx := context.Background()
	dbID := thread.NewIDV1(thread.Raw, 16)
	dbToken := thread.Token("token")
	bucketKey := "buckKey"
	jid := "jobID1"
	bucketRoot, _ := cid.Decode("QmSnuWmxptJZdLJpKRarxBMS2Ju2oANVrgbr2xWbie9b2D")
	_, key, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	err = col.CreateArchive(ctx, dbID, dbToken, bucketKey, jid, bucketRoot, thread.NewLibp2pPubKey(key))
	require.NoError(t, err)

	cause := "all good"
	err = col.Finalize(ctx, jid, cause)
	require.NoError(t, err)

	ta, err := col.Get(ctx, jid)
	require.NoError(t, err)
	require.False(t, ta.Active)
	require.Equal(t, cause, ta.Cause)

	tas, err := col.GetReadyToCheck(ctx, 10)
	require.NoError(t, err)
	require.Equal(t, 0, len(tas))
}

func TestArchiveTracking_Reschedule(t *testing.T) {
	db := newDB(t)
	col, err := NewArchiveTracking(context.Background(), db)
	require.NoError(t, err)
	ctx := context.Background()

	dbID := thread.NewIDV1(thread.Raw, 16)
	dbToken := thread.Token("token")
	bucketKey := "buckKey"
	jid := "jobID1"
	bucketRoot, _ := cid.Decode("QmSnuWmxptJZdLJpKRarxBMS2Ju2oANVrgbr2xWbie9b2D")
	_, key, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	err = col.CreateArchive(ctx, dbID, dbToken, bucketKey, jid, bucketRoot, thread.NewLibp2pPubKey(key))
	require.NoError(t, err)

	err = col.Reschedule(ctx, jid, time.Hour+time.Second*5, "retry me")
	require.NoError(t, err)

	ta, err := col.Get(ctx, jid)
	require.NoError(t, err)
	require.True(t, time.Until(ta.ReadyAt) > time.Hour)
	require.True(t, ta.Active)

}
