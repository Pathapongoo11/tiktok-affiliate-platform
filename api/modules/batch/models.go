package batch

import (
	"time"

	"github.com/google/uuid"
)

// PlanRequest is the payload for POST /api/batch/plan.
//
// Given a set of products, the planner lays out a content calendar across
// DaysAhead days at PostsPerDay slots/day, assigning each slot a product and a
// generated hook + caption + hashtags. CreatePosts persists each item as a
// scheduled draft post when true.
type PlanRequest struct {
	ProductIDs  []uuid.UUID `json:"product_ids"`
	DaysAhead   int         `json:"days_ahead"`    // calendar length (default 7)
	PostsPerDay int         `json:"posts_per_day"` // slots per day (default 3)
	CreatePosts bool        `json:"create_posts"`  // also persist as scheduled drafts
}

// PlanItem is a single scheduled piece of content.
type PlanItem struct {
	ProductID   uuid.UUID  `json:"product_id"`
	ProductName string     `json:"product_name"`
	ScheduledAt time.Time  `json:"scheduled_at"`
	Hook        string     `json:"hook"`    // 3-second opening line (Thai)
	Script      string     `json:"script"`  // short voice/caption script (Thai)
	Caption     string     `json:"caption"` // TikTok caption (Thai)
	Hashtags    []string   `json:"hashtags"`
	PostID      *uuid.UUID `json:"post_id,omitempty"` // set when CreatePosts=true
}

// PlanResponse is the generated calendar.
type PlanResponse struct {
	Items        []PlanItem `json:"items"`
	TotalItems   int        `json:"total_items"`
	DaysAhead    int        `json:"days_ahead"`
	PostsPerDay  int        `json:"posts_per_day"`
	PostsCreated int        `json:"posts_created"`
}
