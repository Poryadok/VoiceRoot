package registry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var ErrDiagnosticsDenied = errors.New("owner diagnostics denied")

const maxDiagnosticAuditEvents = 20

type OwnerDiagnostics struct {
	ApplicationID        uuid.UUID                `json:"application_id"`
	ApplicationStatus    string                   `json:"application_status"`
	ApplicationRevision  int64                    `json:"application_revision"`
	ApplicationAdmission string                   `json:"application_admission"`
	ProviderAdmission    string                   `json:"provider_admission"`
	Environments         []EnvironmentDiagnostics `json:"environments"`
	Quota                QuotaDiagnostics         `json:"quota"`
	RecentAudit          []DiagnosticAudit        `json:"recent_audit"`
}

type EnvironmentDiagnostics struct {
	ID                 uuid.UUID                 `json:"environment_id"`
	Kind               string                    `json:"kind"`
	Status             string                    `json:"status"`
	Revision           int64                     `json:"revision"`
	ProviderAssertions []ProviderAssertion       `json:"provider_assertions"`
	Installations      []InstallationDiagnostics `json:"installations"`
}

type ProviderAssertion struct {
	Provider string `json:"provider"`
	Source   string `json:"source"`
}

type InstallationDiagnostics struct {
	ID                     uuid.UUID `json:"installation_id"`
	Status                 string    `json:"status"`
	RegistrationValidation string    `json:"registration_validation"`
	Source                 string    `json:"source"`
}

type QuotaDiagnostics struct {
	Used    int64     `json:"used"`
	Limit   int64     `json:"limit"`
	ResetAt time.Time `json:"reset_at"`
}

type DiagnosticAudit struct {
	ID               uuid.UUID  `json:"id"`
	ActorKind        string     `json:"actor_kind"`
	ActorID          uuid.UUID  `json:"actor_id"`
	ApplicationID    uuid.UUID  `json:"application_id"`
	EnvironmentID    *uuid.UUID `json:"environment_id,omitempty"`
	InstallationID   *uuid.UUID `json:"installation_id,omitempty"`
	Action           string     `json:"operation"`
	Result           string     `json:"result"`
	Source           string     `json:"source"`
	ReasonCode       string     `json:"reason_code,omitempty"`
	QuotaWindowStart *time.Time `json:"quota_window_start,omitempty"`
	DenialCount      int        `json:"denial_count,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
}

func (s *Store) LoadOwnerDiagnostics(ctx context.Context, ownerID, applicationID uuid.UUID) (OwnerDiagnostics, error) {
	if ownerID == uuid.Nil || applicationID == uuid.Nil {
		return OwnerDiagnostics{}, ErrDiagnosticsDenied
	}
	if s == nil || s.Pool == nil {
		return OwnerDiagnostics{}, ErrRegistryUnavailable
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return OwnerDiagnostics{}, fmt.Errorf("begin owner diagnostics: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	diagnostics := OwnerDiagnostics{
		ApplicationID:     applicationID,
		ProviderAdmission: "not_verified",
		Environments:      make([]EnvironmentDiagnostics, 0),
		RecentAudit:       make([]DiagnosticAudit, 0, maxDiagnosticAuditEvents),
		Quota:             QuotaDiagnostics{Limit: 120},
	}
	err = tx.QueryRow(ctx, `SELECT status, revision FROM applications WHERE id=$1 AND owner_account_id=$2`,
		applicationID, ownerID).Scan(&diagnostics.ApplicationStatus, &diagnostics.ApplicationRevision)
	if errors.Is(err, pgx.ErrNoRows) {
		return OwnerDiagnostics{}, ErrDiagnosticsDenied
	}
	if err != nil {
		return OwnerDiagnostics{}, fmt.Errorf("read owner application diagnostics: %w", err)
	}
	var operatorAdmission bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM registry_audit
		WHERE application_id=$1 AND actor_kind='operator' AND action='approve_sandbox'
		AND source='operator_approved' AND result='success')`, applicationID).Scan(&operatorAdmission); err != nil {
		return OwnerDiagnostics{}, fmt.Errorf("read application admission provenance: %w", err)
	}
	if operatorAdmission {
		diagnostics.ApplicationAdmission = "operator_approved"
	} else {
		diagnostics.ApplicationAdmission = "not_admitted"
	}
	now := s.now()
	currentWindow := now.Truncate(time.Minute)
	diagnostics.Quota.ResetAt = currentWindow.Add(time.Minute)
	var quotaWindow time.Time
	var quotaUsed int64
	err = tx.QueryRow(ctx, `SELECT window_start, request_count FROM app_quota_windows WHERE application_id=$1`, applicationID).
		Scan(&quotaWindow, &quotaUsed)
	if err == nil && quotaWindow.Equal(currentWindow) {
		diagnostics.Quota.Used = quotaUsed
	} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return OwnerDiagnostics{}, fmt.Errorf("read application quota diagnostics: %w", err)
	}

	rows, err := tx.Query(ctx, `SELECT id, kind, status, revision, provider_policy FROM environments
		WHERE application_id=$1 ORDER BY kind`, applicationID)
	if err != nil {
		return OwnerDiagnostics{}, fmt.Errorf("list application environments: %w", err)
	}
	for rows.Next() {
		var environment EnvironmentDiagnostics
		var policy []byte
		if err := rows.Scan(&environment.ID, &environment.Kind, &environment.Status, &environment.Revision, &policy); err != nil {
			rows.Close()
			return OwnerDiagnostics{}, fmt.Errorf("read environment diagnostics: %w", err)
		}
		var asserted struct {
			Providers []string `json:"providers"`
		}
		if err := json.Unmarshal(policy, &asserted); err != nil {
			rows.Close()
			return OwnerDiagnostics{}, fmt.Errorf("decode provider assertions: %w", err)
		}
		for _, provider := range asserted.Providers {
			environment.ProviderAssertions = append(environment.ProviderAssertions,
				ProviderAssertion{Provider: provider, Source: "developer_asserted"})
		}
		if environment.ProviderAssertions == nil {
			environment.ProviderAssertions = make([]ProviderAssertion, 0)
		}
		diagnostics.Environments = append(diagnostics.Environments, environment)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return OwnerDiagnostics{}, fmt.Errorf("iterate environment diagnostics: %w", err)
	}
	rows.Close()
	for i := range diagnostics.Environments {
		environment := &diagnostics.Environments[i]
		installationRows, err := tx.Query(ctx, `SELECT id, status FROM installations
			WHERE application_id=$1 AND environment_id=$2 ORDER BY created_at DESC, id DESC`, applicationID, environment.ID)
		if err != nil {
			return OwnerDiagnostics{}, fmt.Errorf("list installation diagnostics: %w", err)
		}
		for installationRows.Next() {
			var installation InstallationDiagnostics
			if err := installationRows.Scan(&installation.ID, &installation.Status); err != nil {
				installationRows.Close()
				return OwnerDiagnostics{}, fmt.Errorf("read installation diagnostics: %w", err)
			}
			installation.RegistrationValidation = "passed"
			installation.Source = "developer_asserted"
			environment.Installations = append(environment.Installations, installation)
		}
		if err := installationRows.Err(); err != nil {
			installationRows.Close()
			return OwnerDiagnostics{}, fmt.Errorf("iterate installation diagnostics: %w", err)
		}
		installationRows.Close()
		if environment.Installations == nil {
			environment.Installations = make([]InstallationDiagnostics, 0)
		}
	}

	auditRows, err := tx.Query(ctx, `SELECT id, actor_kind, actor_id, application_id, environment_id,
		installation_id, action, result, source, reason_code, quota_window_start, denial_count, created_at
		FROM registry_audit WHERE application_id=$1 ORDER BY created_at DESC, id DESC LIMIT $2`,
		applicationID, maxDiagnosticAuditEvents)
	if err != nil {
		return OwnerDiagnostics{}, fmt.Errorf("list owner audit diagnostics: %w", err)
	}
	for auditRows.Next() {
		var event DiagnosticAudit
		var environmentID, installationID pgtype.UUID
		var reasonCode pgtype.Text
		var quotaStart pgtype.Timestamptz
		if err := auditRows.Scan(&event.ID, &event.ActorKind, &event.ActorID, &event.ApplicationID,
			&environmentID, &installationID, &event.Action, &event.Result, &event.Source,
			&reasonCode, &quotaStart, &event.DenialCount, &event.CreatedAt); err != nil {
			auditRows.Close()
			return OwnerDiagnostics{}, fmt.Errorf("read owner audit diagnostics: %w", err)
		}
		if environmentID.Valid {
			id := uuid.UUID(environmentID.Bytes)
			event.EnvironmentID = &id
		}
		if installationID.Valid {
			id := uuid.UUID(installationID.Bytes)
			event.InstallationID = &id
		}
		if reasonCode.Valid {
			event.ReasonCode = reasonCode.String
		}
		if quotaStart.Valid {
			start := quotaStart.Time
			event.QuotaWindowStart = &start
		}
		diagnostics.RecentAudit = append(diagnostics.RecentAudit, event)
	}
	if err := auditRows.Err(); err != nil {
		auditRows.Close()
		return OwnerDiagnostics{}, fmt.Errorf("iterate owner audit diagnostics: %w", err)
	}
	auditRows.Close()
	if err := tx.Commit(ctx); err != nil {
		return OwnerDiagnostics{}, fmt.Errorf("commit owner diagnostics: %w", err)
	}
	return diagnostics, nil
}
