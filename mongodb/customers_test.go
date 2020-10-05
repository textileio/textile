package mongodb_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	. "github.com/textileio/textile/v2/mongodb"
)

const (
	mib = 1048576
)

func TestCustomers_CreateCustomer(t *testing.T) {
	db := newDB(t)
	col, err := NewCustomers(context.Background(), db)
	require.NoError(t, err)

	created, err := col.CreateCustomer(context.Background(), "jon@doe.com", "cusID", "subID")
	require.NoError(t, err)
	assert.Equal(t, "jon@doe.com", created.Email)
	assert.Equal(t, "cusID", created.ID)
	assert.Equal(t, "subID", created.SubscriptionID)
}

func TestCustomers_Get(t *testing.T) {
	db := newDB(t)
	col, err := NewCustomers(context.Background(), db)
	require.NoError(t, err)

	created, err := col.CreateCustomer(context.Background(), "jon@doe.com", "cusID", "subID")
	require.NoError(t, err)

	got, err := col.Get(context.Background(), created.ID)
	require.NoError(t, err)
	assert.Equal(t, "jon@doe.com", got.Email)
	assert.Equal(t, "cusID", got.ID)
	assert.Equal(t, "subID", got.SubscriptionID)
}

func TestCustomers_GetByEmail(t *testing.T) {
	db := newDB(t)
	col, err := NewCustomers(context.Background(), db)
	require.NoError(t, err)

	created, err := col.CreateCustomer(context.Background(), "jon@doe.com", "cusID", "subID")
	require.NoError(t, err)

	got, err := col.GetByEmail(context.Background(), "jon@doe.com")
	require.NoError(t, err)
	assert.Equal(t, created.ID, got.ID)

	_, err = col.GetByEmail(context.Background(), "jane@doe.com")
	require.Error(t, err)
}

func TestCustomers_SetStoredDataSize(t *testing.T) {
	db := newDB(t)
	col, err := NewCustomers(context.Background(), db)
	require.NoError(t, err)

	created, err := col.CreateCustomer(context.Background(), "jon@doe.com", "cusID", "subID")
	require.NoError(t, err)

	// Units should round down
	units, changed, err := col.SetStoredDataSize(context.Background(), created.ID, 1*mib)
	require.NoError(t, err)
	assert.Equal(t, 0, int(units))
	assert.False(t, changed)

	// Units should round up
	units, changed, err = col.SetStoredDataSize(context.Background(), created.ID, 30*mib)
	require.NoError(t, err)
	assert.Equal(t, 1, int(units))
	assert.True(t, changed)

	// Units should round down
	units, changed, err = col.SetStoredDataSize(context.Background(), created.ID, 260*mib)
	require.NoError(t, err)
	assert.Equal(t, 5, int(units))
	assert.True(t, changed)
}

func TestCustomers_IncNetworkEgressSize(t *testing.T) {
	db := newDB(t)
	col, err := NewCustomers(context.Background(), db)
	require.NoError(t, err)

	created, err := col.CreateCustomer(context.Background(), "jon@doe.com", "cusID", "subID")
	require.NoError(t, err)

	// Add some egress under unit size
	units, err := col.IncNetworkEgressSize(context.Background(), created.ID, 1*mib)
	require.NoError(t, err)
	assert.Equal(t, 0, int(units))

	// Add some egress to reach unit size
	units, err = col.IncNetworkEgressSize(context.Background(), created.ID, 99*mib)
	require.NoError(t, err)
	assert.Equal(t, 1, int(units))

	// Add a bunch of egress
	units, err = col.IncNetworkEgressSize(context.Background(), created.ID, 1234*mib)
	require.NoError(t, err)
	assert.Equal(t, 12, int(units))

	// Check remainder by adding enough egress to reach one more unit
	units, err = col.IncNetworkEgressSize(context.Background(), created.ID, 66*mib)
	require.NoError(t, err)
	assert.Equal(t, 1, int(units))
}

func TestCustomers_IncInstanceReadsCount(t *testing.T) {
	db := newDB(t)
	col, err := NewCustomers(context.Background(), db)
	require.NoError(t, err)

	created, err := col.CreateCustomer(context.Background(), "jon@doe.com", "cusID", "subID")
	require.NoError(t, err)

	// Add some egress under unit size
	units, err := col.IncInstanceReadsCount(context.Background(), created.ID, 1)
	require.NoError(t, err)
	assert.Equal(t, 0, int(units))

	// Add some egress to reach unit size
	units, err = col.IncInstanceReadsCount(context.Background(), created.ID, 9999)
	require.NoError(t, err)
	assert.Equal(t, 1, int(units))

	// Add a bunch of egress
	units, err = col.IncInstanceReadsCount(context.Background(), created.ID, 123456)
	require.NoError(t, err)
	assert.Equal(t, 12, int(units))

	// Check remainder by adding enough egress to reach one more unit
	units, err = col.IncInstanceReadsCount(context.Background(), created.ID, 6544)
	require.NoError(t, err)
	assert.Equal(t, 1, int(units))
}

func TestCustomers_IncInstanceWritesCount(t *testing.T) {
	db := newDB(t)
	col, err := NewCustomers(context.Background(), db)
	require.NoError(t, err)

	created, err := col.CreateCustomer(context.Background(), "jon@doe.com", "cusID", "subID")
	require.NoError(t, err)

	// Add some egress under unit size
	units, err := col.IncInstanceWritesCount(context.Background(), created.ID, 1)
	require.NoError(t, err)
	assert.Equal(t, 0, int(units))

	// Add some egress to reach unit size
	units, err = col.IncInstanceWritesCount(context.Background(), created.ID, 4999)
	require.NoError(t, err)
	assert.Equal(t, 1, int(units))

	// Add a bunch of egress
	units, err = col.IncInstanceWritesCount(context.Background(), created.ID, 123456)
	require.NoError(t, err)
	assert.Equal(t, 24, int(units))

	// Check remainder by adding enough egress to reach one more unit
	units, err = col.IncInstanceWritesCount(context.Background(), created.ID, 3456)
	require.NoError(t, err)
	assert.Equal(t, 1, int(units))
}

func TestCustomers_Delete(t *testing.T) {
	db := newDB(t)
	col, err := NewCustomers(context.Background(), db)
	require.NoError(t, err)

	created, err := col.CreateCustomer(context.Background(), "jon@doe.com", "cusID", "subID")
	require.NoError(t, err)

	err = col.Delete(context.Background(), created.ID)
	require.NoError(t, err)
	_, err = col.Get(context.Background(), created.ID)
	require.Error(t, err)
}
