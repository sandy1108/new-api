package clientaccess

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newClientAccessTestStore(t *testing.T) *Store {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.Channel{}))
	require.NoError(t, model.MigrateClientAccessTables(db))
	return NewStore(db)
}

func TestFindCredentialByTokenKeepsDatabaseFailureDistinctFromInvalidToken(t *testing.T) {
	store := newClientAccessTestStore(t)
	sqlDB, err := store.DB.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	_, err = store.FindCredentialByToken("napi_client_validshape")
	require.Error(t, err)
	require.False(t, errors.Is(err, ErrCredentialUnavailable))
}

func addCodexChannel(t *testing.T, store *Store, id int, channelType int, enabled bool) {
	t.Helper()
	status := common.ChannelStatusManuallyDisabled
	if enabled {
		status = common.ChannelStatusEnabled
	}
	require.NoError(t, store.DB.Create(&model.Channel{
		Id: id, Name: "test", Type: channelType, Status: status,
		Key: `{"access_token":"access","refresh_token":"refresh","account_id":"account"}`,
	}).Error)
}

func TestCreateCredentialStoresHashOnlyAndOneGrant(t *testing.T) {
	store := newClientAccessTestStore(t)
	addCodexChannel(t, store, 17, constant.ChannelTypeCodex, true)

	issued, err := store.CreateCredential(1, CreateInput{
		Name: "Android", ExpiredTime: -1,
		Grants: []GrantInput{{Scope: "codex.quota.read", ResourceID: "17"}},
	})
	require.NoError(t, err)
	require.Contains(t, issued.Token, "napi_client_")
	require.GreaterOrEqual(t, len(issued.Token), 54)

	var saved model.ClientAccessCredential
	require.NoError(t, store.DB.First(&saved, issued.Credential.Id).Error)
	sum := sha256.Sum256([]byte(issued.Token))
	require.Equal(t, hex.EncodeToString(sum[:]), saved.TokenHash)
	require.NotContains(t, saved.TokenHash, issued.Token)
	require.EqualValues(t, 1, saved.Version)
	var grants []model.ClientAccessGrant
	require.NoError(t, store.DB.Where("credential_id = ?", saved.Id).Find(&grants).Error)
	require.Len(t, grants, 1)
	require.Equal(t, "channel", grants[0].ResourceType)
	require.Equal(t, "17", grants[0].ResourceId)
}

func TestCreateCredentialRejectsUnknownAndInvalidResourcesWithoutPartialRecord(t *testing.T) {
	store := newClientAccessTestStore(t)
	addCodexChannel(t, store, 17, constant.ChannelTypeCodex, true)
	addCodexChannel(t, store, 18, constant.ChannelTypeOpenAI, true)
	addCodexChannel(t, store, 19, constant.ChannelTypeCodex, false)

	cases := []CreateInput{
		{Name: "empty", ExpiredTime: -1},
		{Name: "unknown", ExpiredTime: -1, Grants: []GrantInput{{Scope: "anything", ResourceID: "17"}}},
		{Name: "wildcard", ExpiredTime: -1, Grants: []GrantInput{{Scope: "*", ResourceID: "17"}}},
		{Name: "wrong type", ExpiredTime: -1, Grants: []GrantInput{{Scope: "codex.quota.read", ResourceID: "18"}}},
		{Name: "disabled", ExpiredTime: -1, Grants: []GrantInput{{Scope: "codex.quota.read", ResourceID: "19"}}},
		{Name: "missing", ExpiredTime: -1, Grants: []GrantInput{{Scope: "codex.quota.read", ResourceID: "999"}}},
		{Name: "duplicate", ExpiredTime: -1, Grants: []GrantInput{{Scope: "codex.quota.read", ResourceID: "17"}, {Scope: "codex.quota.read", ResourceID: "17"}}},
		{Name: "expired", ExpiredTime: time.Now().Unix() - 1, Grants: []GrantInput{{Scope: "codex.quota.read", ResourceID: "17"}}},
	}
	for _, input := range cases {
		_, err := store.CreateCredential(1, input)
		require.Error(t, err, input.Name)
	}
	var count int64
	require.NoError(t, store.DB.Model(&model.ClientAccessCredential{}).Count(&count).Error)
	require.Zero(t, count)
}

func TestUpdateGrantsUsesVersionAndRollsBackInvalidReplacement(t *testing.T) {
	store := newClientAccessTestStore(t)
	addCodexChannel(t, store, 17, constant.ChannelTypeCodex, true)
	addCodexChannel(t, store, 18, constant.ChannelTypeCodex, true)
	issued, err := store.CreateCredential(1, CreateInput{Name: "Android", ExpiredTime: -1,
		Grants: []GrantInput{{Scope: ScopeCodexQuotaRead, ResourceID: "17"}}})
	require.NoError(t, err)

	_, err = store.UpdateGrants(issued.Credential.Id, 1, []GrantInput{{Scope: ScopeCodexQuotaRead, ResourceID: "999"}})
	require.Error(t, err)
	grants, err := store.GetGrants(issued.Credential.Id)
	require.NoError(t, err)
	require.Len(t, grants, 1)
	require.Equal(t, "17", grants[0].ResourceId)

	updated, err := store.UpdateGrants(issued.Credential.Id, 1, []GrantInput{{Scope: ScopeCodexQuotaRead, ResourceID: "18"}})
	require.NoError(t, err)
	require.EqualValues(t, 2, updated.Version)
	_, err = store.UpdateGrants(issued.Credential.Id, 1, nil)
	require.ErrorIs(t, err, ErrVersionConflict)
	grants, err = store.GetGrants(issued.Credential.Id)
	require.NoError(t, err)
	require.Len(t, grants, 1)
	require.Equal(t, "18", grants[0].ResourceId)

	_, err = store.UpdateGrants(issued.Credential.Id, 2, nil)
	require.NoError(t, err)
	grants, err = store.GetGrants(issued.Credential.Id)
	require.NoError(t, err)
	require.Empty(t, grants)
}

func TestRotateAndRevokeInvalidateOldToken(t *testing.T) {
	store := newClientAccessTestStore(t)
	addCodexChannel(t, store, 17, constant.ChannelTypeCodex, true)
	issued, err := store.CreateCredential(1, CreateInput{Name: "Android", ExpiredTime: -1,
		Grants: []GrantInput{{Scope: ScopeCodexQuotaRead, ResourceID: "17"}}})
	require.NoError(t, err)

	rotated, err := store.RotateCredential(issued.Credential.Id, 1)
	require.NoError(t, err)
	require.NotEqual(t, issued.Token, rotated.Token)
	_, err = store.FindCredentialByToken(issued.Token)
	require.Error(t, err)
	_, err = store.FindCredentialByToken(rotated.Token)
	require.NoError(t, err)

	_, err = store.SetStatus(issued.Credential.Id, 2, CredentialStatusDisabled)
	require.NoError(t, err)
	_, err = store.FindCredentialByToken(rotated.Token)
	require.Error(t, err)
	_, err = store.SetStatus(issued.Credential.Id, 3, CredentialStatusEnabled)
	require.NoError(t, err)
	_, err = store.FindCredentialByToken(rotated.Token)
	require.NoError(t, err)
	_, err = store.SetStatus(issued.Credential.Id, 4, CredentialStatusRevoked)
	require.NoError(t, err)
	_, err = store.FindCredentialByToken(rotated.Token)
	require.Error(t, err)
	_, err = store.SetStatus(issued.Credential.Id, 5, CredentialStatusEnabled)
	require.Error(t, err)
	_, err = store.RotateCredential(issued.Credential.Id, 5)
	require.Error(t, err)
}

func TestAuthorizeScopeRevalidatesChannelAfterGrantWasCreated(t *testing.T) {
	store := newClientAccessTestStore(t)
	addCodexChannel(t, store, 17, constant.ChannelTypeCodex, true)
	issued, err := store.CreateCredential(1, CreateInput{Name: "Android", ExpiredTime: -1,
		Grants: []GrantInput{{Scope: ScopeCodexQuotaRead, ResourceID: "17"}}})
	require.NoError(t, err)

	resourceID, err := store.AuthorizeScope(issued.Credential.Id, ScopeCodexQuotaRead)
	require.NoError(t, err)
	require.Equal(t, "17", resourceID)
	_, err = store.AuthorizeScope(issued.Credential.Id, "other.scope")
	require.ErrorIs(t, err, ErrScopeDenied)

	require.NoError(t, store.DB.Model(&model.Channel{}).Where("id = ?", 17).Update("status", common.ChannelStatusManuallyDisabled).Error)
	_, err = store.AuthorizeScope(issued.Credential.Id, ScopeCodexQuotaRead)
	require.ErrorIs(t, err, ErrResourceUnavailable)
}

func TestTouchLastUsedWritesAtMostOncePerFiveMinutes(t *testing.T) {
	store := newClientAccessTestStore(t)
	addCodexChannel(t, store, 17, constant.ChannelTypeCodex, true)
	issued, err := store.CreateCredential(1, CreateInput{Name: "Android", ExpiredTime: -1,
		Grants: []GrantInput{{Scope: ScopeCodexQuotaRead, ResourceID: "17"}}})
	require.NoError(t, err)
	require.NoError(t, store.TouchLastUsed(issued.Credential.Id, 1000))
	require.NoError(t, store.TouchLastUsed(issued.Credential.Id, 1100))
	var credential model.ClientAccessCredential
	require.NoError(t, store.DB.First(&credential, issued.Credential.Id).Error)
	require.EqualValues(t, 1000, credential.LastUsedTime)
	require.NoError(t, store.TouchLastUsed(issued.Credential.Id, 1301))
	require.NoError(t, store.DB.First(&credential, issued.Credential.Id).Error)
	require.EqualValues(t, 1301, credential.LastUsedTime)
}
