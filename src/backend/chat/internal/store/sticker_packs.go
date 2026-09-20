package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrStickerPackNotFound  = errors.New("sticker pack not found")
	ErrStickerPackForbidden = errors.New("sticker pack is not available to this profile")
	ErrSystemStickerPack    = errors.New("system sticker packs cannot be uninstalled")
)

type StickerRow struct {
	ID, FileID               uuid.UUID
	Emoji                    *string
	SortOrder, Width, Height int32
}
type StickerPackRow struct {
	ID                      uuid.UUID
	Title                   string
	ThumbFileID             *uuid.UUID
	IsSystem, IsPremium     bool
	CreatorProfileID        *uuid.UUID
	StickerCount, SortOrder int32
	Stickers                []StickerRow
}

func (s *DMStore) ListInstalledStickerPacks(ctx context.Context, profileID uuid.UUID) ([]StickerPackRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("dm store: pool not configured")
	}
	rows, err := s.Pool.Query(ctx, `SELECT p.id, p.title, p.thumb_file_id, p.is_system, p.is_premium, p.creator_profile_id, p.sticker_count, i.sort_order FROM profile_installed_packs i JOIN sticker_packs p ON p.id=i.pack_id WHERE i.profile_id=$1 ORDER BY i.sort_order, i.installed_at`, profileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var packs []StickerPackRow
	for rows.Next() {
		var p StickerPackRow
		if err := rows.Scan(&p.ID, &p.Title, &p.ThumbFileID, &p.IsSystem, &p.IsPremium, &p.CreatorProfileID, &p.StickerCount, &p.SortOrder); err != nil {
			return nil, err
		}
		if err = s.loadStickers(ctx, &p); err != nil {
			return nil, err
		}
		packs = append(packs, p)
	}
	return packs, rows.Err()
}

func (s *DMStore) GetStickerPack(ctx context.Context, profileID, packID uuid.UUID) (*StickerPackRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("dm store: pool not configured")
	}
	p, err := s.findStickerPack(ctx, profileID, packID)
	if err != nil {
		return nil, err
	}
	if p.IsSystem || (p.CreatorProfileID != nil && *p.CreatorProfileID == profileID) || p.SortOrder >= 0 {
		return p, nil
	}
	return nil, ErrStickerPackForbidden
}

func (s *DMStore) InstallStickerPack(ctx context.Context, profileID, packID uuid.UUID) (*StickerPackRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("dm store: pool not configured")
	}
	p, err := s.findStickerPack(ctx, profileID, packID)
	if err != nil {
		return nil, err
	}
	if !p.IsSystem && (p.CreatorProfileID == nil || *p.CreatorProfileID != profileID) {
		return nil, ErrStickerPackForbidden
	}
	_, err = s.Pool.Exec(ctx, `INSERT INTO profile_installed_packs (profile_id, pack_id, sort_order) SELECT $1,$2,COALESCE(MAX(sort_order),-1)+1 FROM profile_installed_packs WHERE profile_id=$1 ON CONFLICT (profile_id,pack_id) DO NOTHING`, profileID, packID)
	if err != nil {
		return nil, err
	}
	return s.findStickerPack(ctx, profileID, packID)
}

func (s *DMStore) UninstallStickerPack(ctx context.Context, profileID, packID uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return errors.New("dm store: pool not configured")
	}
	p, err := s.findStickerPack(ctx, profileID, packID)
	if err != nil {
		return err
	}
	if p.IsSystem {
		return ErrSystemStickerPack
	}
	if p.CreatorProfileID == nil || *p.CreatorProfileID != profileID {
		return ErrStickerPackForbidden
	}
	ct, err := s.Pool.Exec(ctx, `DELETE FROM profile_installed_packs WHERE profile_id=$1 AND pack_id=$2`, profileID, packID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrStickerPackNotFound
	}
	return nil
}

func (s *DMStore) findStickerPack(ctx context.Context, profileID, packID uuid.UUID) (*StickerPackRow, error) {
	p := &StickerPackRow{SortOrder: -1}
	err := s.Pool.QueryRow(ctx, `SELECT p.id,p.title,p.thumb_file_id,p.is_system,p.is_premium,p.creator_profile_id,p.sticker_count,COALESCE(i.sort_order,-1) FROM sticker_packs p LEFT JOIN profile_installed_packs i ON i.pack_id=p.id AND i.profile_id=$1 WHERE p.id=$2`, profileID, packID).Scan(&p.ID, &p.Title, &p.ThumbFileID, &p.IsSystem, &p.IsPremium, &p.CreatorProfileID, &p.StickerCount, &p.SortOrder)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrStickerPackNotFound
	}
	if err != nil {
		return nil, err
	}
	if err = s.loadStickers(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}
func (s *DMStore) loadStickers(ctx context.Context, p *StickerPackRow) error {
	rows, err := s.Pool.Query(ctx, `SELECT id,file_id,emoji,sort_order,width,height FROM stickers WHERE pack_id=$1 ORDER BY sort_order`, p.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var v StickerRow
		if err = rows.Scan(&v.ID, &v.FileID, &v.Emoji, &v.SortOrder, &v.Width, &v.Height); err != nil {
			return err
		}
		p.Stickers = append(p.Stickers, v)
	}
	return rows.Err()
}
