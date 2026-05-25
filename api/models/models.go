package models

import (
	"time"

	"github.com/google/uuid"
)

// --- Domain Models ---

type User struct {
	ID           uuid.UUID  `json:"id"`
	Email        string     `json:"email"`
	PasswordHash string     `json:"-"`
	DisplayName  string     `json:"display_name"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type TikTokAccount struct {
	ID             uuid.UUID  `json:"id"`
	UserID         uuid.UUID  `json:"user_id"`
	TikTokUserID   string     `json:"tiktok_user_id"`
	DisplayName    string     `json:"display_name"`
	AccessToken    string     `json:"-"`
	RefreshToken   string     `json:"-"`
	TokenExpiresAt time.Time  `json:"token_expires_at"`
	IsActive       bool       `json:"is_active"`
}

type Product struct {
	ID            uuid.UUID `json:"id"`
	UserID        uuid.UUID `json:"user_id"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	Price         float64   `json:"price"`
	CommissionRate float64  `json:"commission_rate"`
	Category      string    `json:"category"`
	ShopProductID string    `json:"shop_product_id"`
	ImageURLs     []string  `json:"image_urls"`
	Score         float64   `json:"score"`
	IsActive      bool      `json:"is_active"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type Post struct {
	ID               uuid.UUID  `json:"id"`
	UserID           uuid.UUID  `json:"user_id"`
	TikTokAccountID  *uuid.UUID `json:"tiktok_account_id,omitempty"`
	ProductID        *uuid.UUID `json:"product_id,omitempty"`
	Title            string     `json:"title"`
	Caption          string     `json:"caption"`
	Hashtags         []string   `json:"hashtags"`
	VideoPath        string     `json:"video_path"`
	TikTokVideoID    string     `json:"tiktok_video_id"`
	Status           string     `json:"status"`
	ScheduledAt      *time.Time `json:"scheduled_at,omitempty"`
	PublishedAt      *time.Time `json:"published_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	// Joined fields
	Product          *Product   `json:"product,omitempty"`
}

type VideoJob struct {
	ID              uuid.UUID  `json:"id"`
	PostID          *uuid.UUID `json:"post_id,omitempty"`
	UserID          uuid.UUID  `json:"user_id"`
	Status          string     `json:"status"`
	InputImages     []string   `json:"input_images"`
	OverlayText     string     `json:"overlay_text"`
	AudioPath       string     `json:"audio_path"`
	OutputPath      string     `json:"output_path"`
	DurationSeconds int        `json:"duration_seconds"`
	ErrorMessage    string     `json:"error_message"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type Analytics struct {
	ID           uuid.UUID `json:"id"`
	PostID       uuid.UUID `json:"post_id"`
	UserID       uuid.UUID `json:"user_id"`
	Views        int64     `json:"views"`
	Likes        int64     `json:"likes"`
	Comments     int64     `json:"comments"`
	Shares       int64     `json:"shares"`
	BasketClicks int64     `json:"basket_clicks"`
	Orders       int64     `json:"orders"`
	Revenue      float64   `json:"revenue"`
	RecordedAt   time.Time `json:"recorded_at"`
}

// --- Request DTOs ---

type RegisterRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type CreateProductRequest struct {
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Price          float64  `json:"price"`
	CommissionRate float64  `json:"commission_rate"`
	Category       string   `json:"category"`
	ShopProductID  string   `json:"shop_product_id"`
	ImageURLs      []string `json:"image_urls"`
}

type UpdateProductRequest struct {
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Price          float64  `json:"price"`
	CommissionRate float64  `json:"commission_rate"`
	Category       string   `json:"category"`
	ShopProductID  string   `json:"shop_product_id"`
	ImageURLs      []string `json:"image_urls"`
}

type CreatePostRequest struct {
	TikTokAccountID *uuid.UUID `json:"tiktok_account_id,omitempty"`
	ProductID       *uuid.UUID `json:"product_id,omitempty"`
	Title           string     `json:"title"`
	Caption         string     `json:"caption"`
	Hashtags        []string   `json:"hashtags"`
	VideoPath       string     `json:"video_path"`
}

type UpdatePostRequest struct {
	TikTokAccountID *uuid.UUID `json:"tiktok_account_id,omitempty"`
	ProductID       *uuid.UUID `json:"product_id,omitempty"`
	Title           string     `json:"title"`
	Caption         string     `json:"caption"`
	Hashtags        []string   `json:"hashtags"`
	VideoPath       string     `json:"video_path"`
}

type SchedulePostRequest struct {
	ScheduledAt time.Time `json:"scheduled_at"`
}

// --- Response DTOs ---

type AuthResponse struct {
	Token string `json:"token"`
	User  *User  `json:"user"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type PaginatedResponse struct {
	Data  interface{} `json:"data"`
	Total int         `json:"total"`
	Page  int         `json:"page"`
	Limit int         `json:"limit"`
}

type ProductFlags struct {
	Warnings []string `json:"warnings,omitempty"`
}

type ProductWithFlags struct {
	*Product
	Flags ProductFlags `json:"flags,omitempty"`
}

type DashboardStats struct {
	TotalPosts      int     `json:"total_posts"`
	TotalViews      int64   `json:"total_views"`
	TotalLikes      int64   `json:"total_likes"`
	TotalRevenue    float64 `json:"total_revenue"`
	AvgEngagement   float64 `json:"avg_engagement_rate"`
}

type TopPost struct {
	Post  *Post  `json:"post"`
	Views int64  `json:"views"`
}

type ProductPerformance struct {
	Product      *Product `json:"product"`
	BasketClicks int64    `json:"basket_clicks"`
	Orders       int64    `json:"orders"`
	Revenue      float64  `json:"revenue"`
}
