package handler

import (
	"time"

	"github.com/msomdec/stitch-map-2/internal/domain"
	"github.com/msomdec/stitch-map-2/internal/service"
)

// UserDTO is the JSON representation of a user.
type UserDTO struct {
	ID          int64  `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

func toUserDTO(u *domain.User) UserDTO {
	return UserDTO{
		ID:          u.ID,
		Email:       u.Email,
		DisplayName: u.DisplayName,
		CreatedAt:   u.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   u.UpdatedAt.Format(time.RFC3339),
	}
}

// StitchDTO is the JSON representation of a stitch.
type StitchDTO struct {
	ID           int64  `json:"id"`
	Abbreviation string `json:"abbreviation"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	Category     string `json:"category"`
	IsCustom     bool   `json:"isCustom"`
	UserID       *int64 `json:"userId"`
	CreatedAt    string `json:"createdAt"`
}

func toStitchDTO(s domain.Stitch) StitchDTO {
	return StitchDTO{
		ID:           s.ID,
		Abbreviation: s.Abbreviation,
		Name:         s.Name,
		Description:  s.Description,
		Category:     s.Category,
		IsCustom:     s.IsCustom,
		UserID:       s.UserID,
		CreatedAt:    s.CreatedAt.Format(time.RFC3339),
	}
}

func toStitchDTOs(stitches []domain.Stitch) []StitchDTO {
	dtos := make([]StitchDTO, len(stitches))
	for i, s := range stitches {
		dtos[i] = toStitchDTO(s)
	}
	return dtos
}

// StitchEntryDTO is the JSON representation of a stitch entry.
type StitchEntryDTO struct {
	ID                 int64  `json:"id"`
	InstructionGroupID int64  `json:"instructionGroupId"`
	SortOrder          int    `json:"sortOrder"`
	StitchID           int64  `json:"stitchId"`
	Count              int    `json:"count"`
	IntoStitch         string `json:"intoStitch"`
	RepeatCount        int    `json:"repeatCount"`
}

func toStitchEntryDTO(e domain.StitchEntry) StitchEntryDTO {
	return StitchEntryDTO{
		ID:                 e.ID,
		InstructionGroupID: e.InstructionGroupID,
		SortOrder:          e.SortOrder,
		StitchID:           e.StitchID,
		Count:              e.Count,
		IntoStitch:         e.IntoStitch,
		RepeatCount:        e.RepeatCount,
	}
}

func toStitchEntryDTOs(entries []domain.StitchEntry) []StitchEntryDTO {
	dtos := make([]StitchEntryDTO, len(entries))
	for i, e := range entries {
		dtos[i] = toStitchEntryDTO(e)
	}
	return dtos
}

// InstructionGroupDTO is the JSON representation of an instruction group.
type InstructionGroupDTO struct {
	ID            int64            `json:"id"`
	PatternID     int64            `json:"patternId"`
	SortOrder     int              `json:"sortOrder"`
	Label         string           `json:"label"`
	RepeatCount   int              `json:"repeatCount"`
	StitchEntries []StitchEntryDTO `json:"stitchEntries"`
	ExpectedCount *int             `json:"expectedCount"`
	Notes         string           `json:"notes"`
}

func toInstructionGroupDTO(g domain.InstructionGroup) InstructionGroupDTO {
	return InstructionGroupDTO{
		ID:            g.ID,
		PatternID:     g.PatternID,
		SortOrder:     g.SortOrder,
		Label:         g.Label,
		RepeatCount:   g.RepeatCount,
		StitchEntries: toStitchEntryDTOs(g.StitchEntries),
		ExpectedCount: g.ExpectedCount,
		Notes:         g.Notes,
	}
}

func toInstructionGroupDTOs(groups []domain.InstructionGroup) []InstructionGroupDTO {
	dtos := make([]InstructionGroupDTO, len(groups))
	for i, g := range groups {
		dtos[i] = toInstructionGroupDTO(g)
	}
	return dtos
}

// PatternDTO is the JSON representation of a pattern.
type PatternDTO struct {
	ID                int64                 `json:"id"`
	UserID            int64                 `json:"userId"`
	Name              string                `json:"name"`
	Description       string                `json:"description"`
	PatternType       string                `json:"patternType"`
	HookSize          string                `json:"hookSize"`
	YarnWeight        string                `json:"yarnWeight"`
	Difficulty        string                `json:"difficulty"`
	InstructionGroups []InstructionGroupDTO `json:"instructionGroups"`
	CreatedAt         string                `json:"createdAt"`
	UpdatedAt         string                `json:"updatedAt"`
}

func toPatternDTO(p *domain.Pattern) PatternDTO {
	return PatternDTO{
		ID:                p.ID,
		UserID:            p.UserID,
		Name:              p.Name,
		Description:       p.Description,
		PatternType:       string(p.PatternType),
		HookSize:          p.HookSize,
		YarnWeight:        p.YarnWeight,
		Difficulty:        p.Difficulty,
		InstructionGroups: toInstructionGroupDTOs(p.InstructionGroups),
		CreatedAt:         p.CreatedAt.Format(time.RFC3339),
		UpdatedAt:         p.UpdatedAt.Format(time.RFC3339),
	}
}

func toPatternDTOs(patterns []domain.Pattern) []PatternDTO {
	dtos := make([]PatternDTO, len(patterns))
	for i := range patterns {
		dtos[i] = toPatternDTO(&patterns[i])
	}
	return dtos
}

// WorkSessionDTO is the JSON representation of a work session.
type WorkSessionDTO struct {
	ID                  int64   `json:"id"`
	PatternID           int64   `json:"patternId"`
	UserID              int64   `json:"userId"`
	CurrentGroupIndex   int     `json:"currentGroupIndex"`
	CurrentGroupRepeat  int     `json:"currentGroupRepeat"`
	CurrentStitchIndex  int     `json:"currentStitchIndex"`
	CurrentStitchRepeat int     `json:"currentStitchRepeat"`
	CurrentStitchCount  int     `json:"currentStitchCount"`
	Status              string  `json:"status"`
	StartedAt           string  `json:"startedAt"`
	LastActivityAt      string  `json:"lastActivityAt"`
	CompletedAt         *string `json:"completedAt"`
}

func toWorkSessionDTO(s *domain.WorkSession) WorkSessionDTO {
	dto := WorkSessionDTO{
		ID:                  s.ID,
		PatternID:           s.PatternID,
		UserID:              s.UserID,
		CurrentGroupIndex:   s.CurrentGroupIndex,
		CurrentGroupRepeat:  s.CurrentGroupRepeat,
		CurrentStitchIndex:  s.CurrentStitchIndex,
		CurrentStitchRepeat: s.CurrentStitchRepeat,
		CurrentStitchCount:  s.CurrentStitchCount,
		Status:              s.Status,
		StartedAt:           s.StartedAt.Format(time.RFC3339),
		LastActivityAt:      s.LastActivityAt.Format(time.RFC3339),
	}
	if s.CompletedAt != nil {
		t := s.CompletedAt.Format(time.RFC3339)
		dto.CompletedAt = &t
	}
	return dto
}

func toWorkSessionDTOs(sessions []domain.WorkSession) []WorkSessionDTO {
	dtos := make([]WorkSessionDTO, len(sessions))
	for i := range sessions {
		dtos[i] = toWorkSessionDTO(&sessions[i])
	}
	return dtos
}

// GroupProgressDTO is the JSON representation of progress for a single instruction group.
type GroupProgressDTO struct {
	Label            string `json:"label"`
	RepeatCount      int    `json:"repeatCount"`
	CurrentRepeat    int    `json:"currentRepeat"`
	Status           string `json:"status"`
	CompletedInGroup int    `json:"completedInGroup"`
	TotalInGroup     int    `json:"totalInGroup"`
}

// SessionProgressDTO is the JSON representation of overall session progress.
type SessionProgressDTO struct {
	CompletedStitches int                `json:"completedStitches"`
	TotalStitches     int                `json:"totalStitches"`
	Percentage        float64            `json:"percentage"`
	GroupLabel        string             `json:"groupLabel"`
	GroupRepeatInfo   string             `json:"groupRepeatInfo"`
	CurrentAbbr       string             `json:"currentAbbr"`
	CurrentName       string             `json:"currentName"`
	PrevAbbr          string             `json:"prevAbbr"`
	NextAbbr          string             `json:"nextAbbr"`
	Groups            []GroupProgressDTO `json:"groups"`
}

func toSessionProgressDTO(p service.SessionProgress) SessionProgressDTO {
	groups := make([]GroupProgressDTO, len(p.Groups))
	for i, g := range p.Groups {
		groups[i] = GroupProgressDTO{
			Label:            g.Label,
			RepeatCount:      g.RepeatCount,
			CurrentRepeat:    g.CurrentRepeat,
			Status:           g.Status,
			CompletedInGroup: g.CompletedInGroup,
			TotalInGroup:     g.TotalInGroup,
		}
	}
	return SessionProgressDTO{
		CompletedStitches: p.CompletedStitches,
		TotalStitches:     p.TotalStitches,
		Percentage:        p.Percentage,
		GroupLabel:         p.GroupLabel,
		GroupRepeatInfo:    p.GroupRepeatInfo,
		CurrentAbbr:       p.CurrentAbbr,
		CurrentName:       p.CurrentName,
		PrevAbbr:          p.PrevAbbr,
		NextAbbr:          p.NextAbbr,
		Groups:            groups,
	}
}

// PatternImageDTO is the JSON representation of a pattern image.
type PatternImageDTO struct {
	ID                 int64  `json:"id"`
	InstructionGroupID int64  `json:"instructionGroupId"`
	Filename           string `json:"filename"`
	ContentType        string `json:"contentType"`
	Size               int64  `json:"size"`
	SortOrder          int    `json:"sortOrder"`
	CreatedAt          string `json:"createdAt"`
}

func toPatternImageDTO(img domain.PatternImage) PatternImageDTO {
	return PatternImageDTO{
		ID:                 img.ID,
		InstructionGroupID: img.InstructionGroupID,
		Filename:           img.Filename,
		ContentType:        img.ContentType,
		Size:               img.Size,
		SortOrder:          img.SortOrder,
		CreatedAt:          img.CreatedAt.Format(time.RFC3339),
	}
}

func toPatternImageDTOs(images []domain.PatternImage) []PatternImageDTO {
	dtos := make([]PatternImageDTO, len(images))
	for i, img := range images {
		dtos[i] = toPatternImageDTO(img)
	}
	return dtos
}
