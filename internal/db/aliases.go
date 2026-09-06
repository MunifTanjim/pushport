// Re-exports so callers depend only on internal/db. NewNullString/NewNullInt64
// are wrappers because generic functions cannot be var-aliased.

package db

import (
	"github.com/MunifTanjim/pushport/internal/db/sqlc"
	"github.com/MunifTanjim/pushport/internal/db/types"
)

type Date = types.Date

type NullString = types.NullString

func NewNullString[T string | *string](s T) NullString {
	return types.NewNullString(s)
}

type NullInt64 = types.NullInt64

func NewNullInt64[T int | *int | int64 | *int64](n T) NullInt64 {
	return types.NewNullInt64(n)
}

var (
	ParseDate = types.ParseDate
	NewDate   = types.NewDate
)

type (
	Queries                              = sqlc.Queries
	App                                  = sqlc.App
	Instance                             = sqlc.Instance
	AppUsagePlan                         = sqlc.AppUsagePlan
	InstanceUsagePlan                    = sqlc.InstanceUsagePlan
	CreateAppParams                      = sqlc.CreateAppParams
	UpdateAppParams                      = sqlc.UpdateAppParams
	CreateInstanceParams                 = sqlc.CreateInstanceParams
	UpdateInstanceParams                 = sqlc.UpdateInstanceParams
	UpdateInstanceTokenHashParams        = sqlc.UpdateInstanceTokenHashParams
	UpsertAppCredentialParams            = sqlc.UpsertAppCredentialParams
	DeleteAppCredentialParams            = sqlc.DeleteAppCredentialParams
	ListAppCredentialsRow                = sqlc.ListAppCredentialsRow
	UpdateAppKeyVersionParams            = sqlc.UpdateAppKeyVersionParams
	SetAppPublicParams                   = sqlc.SetAppPublicParams
	SetAppTokenParams                    = sqlc.SetAppTokenParams
	SetAppTurnstileParams                = sqlc.SetAppTurnstileParams
	CreateAppUsagePlanParams             = sqlc.CreateAppUsagePlanParams
	UpdateAppUsagePlanParams             = sqlc.UpdateAppUsagePlanParams
	AssignAppUsagePlanParams             = sqlc.AssignAppUsagePlanParams
	CreateInstanceUsagePlanParams        = sqlc.CreateInstanceUsagePlanParams
	UpdateInstanceUsagePlanParams        = sqlc.UpdateInstanceUsagePlanParams
	AssignInstanceUsagePlanParams        = sqlc.AssignInstanceUsagePlanParams
	DeleteInstanceUsagePlanParams        = sqlc.DeleteInstanceUsagePlanParams
	CreateDefaultInstanceUsagePlanParams = sqlc.CreateDefaultInstanceUsagePlanParams
	CreateDefaultAppUsagePlanParams      = sqlc.CreateDefaultAppUsagePlanParams
	UpsertUsageCounterParams             = sqlc.UpsertUsageCounterParams
	ListUsageCountersForDayRow           = sqlc.ListUsageCountersForDayRow
	ServerSetting                        = sqlc.Setting
	UpdateServerSettingParams            = sqlc.UpdateServerSettingParams
)
