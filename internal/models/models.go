package models

import "time"

type Outage struct {
	ShowCurOutage   bool
	StartDate       time.Time
	EndDate         time.Time
	Text            string
	Type            string
	UpdateTimestamp time.Time
}

type SavedInfo struct {
	LastMessageID       int        `json:"last_message_id"`
	PrevText            *string    `json:"prev_text"`
	PrevStartDate       *time.Time `json:"prev_start_date"`
	PrevEndDate         *time.Time `json:"prev_end_date"`
	PrevUpdateTimestamp *time.Time `json:"prev_update_timestamp"`
}
