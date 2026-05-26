package tiktok

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Scheduler struct {
	db     *pgxpool.Pool
	client *Client
	repo   *Repository
}

func NewScheduler(db *pgxpool.Pool, client *Client) *Scheduler {
	return &Scheduler{db: db, client: client, repo: NewRepository(db)}
}

func (s *Scheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	log.Println("[Scheduler] Started — polling every 60s")
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.processDuePosts(ctx)
		}
	}
}

func (s *Scheduler) processDuePosts(ctx context.Context) {
	posts, err := s.repo.GetDuePosts(ctx)
	if err != nil {
		log.Printf("[Scheduler] fetch error: %v", err)
		return
	}
	for _, post := range posts {
		s.repo.UpdatePostStatus(ctx, post.ID, "posting", "", "") //nolint:errcheck

		if post.TikTokAccountID == nil {
			s.repo.UpdatePostStatus(ctx, post.ID, "failed", "", "no tiktok account linked") //nolint:errcheck
			continue
		}

		account, err := s.repo.GetTikTokAccountByID(ctx, *post.TikTokAccountID)
		if err != nil {
			s.repo.UpdatePostStatus(ctx, post.ID, "failed", "", "no tiktok account") //nolint:errcheck
			continue
		}

		if time.Now().After(account.TokenExpiresAt) {
			newTok, err := s.client.RefreshAccessToken(ctx, account.RefreshToken)
			if err != nil {
				s.repo.UpdatePostStatus(ctx, post.ID, "failed", "", "token refresh failed") //nolint:errcheck
				continue
			}
			s.repo.UpdateAccountToken(ctx, account.ID, newTok.AccessToken, newTok.RefreshToken, //nolint:errcheck
				time.Now().Add(time.Duration(newTok.ExpiresIn)*time.Second))
			account.AccessToken = newTok.AccessToken
		}

		resp, err := s.client.PostVideo(ctx, PostVideoRequest{
			AccessToken:  account.AccessToken,
			VideoPath:    post.VideoPath,
			Caption:      post.Caption,
			Hashtags:     post.Hashtags,
			PrivacyLevel: "PUBLIC_TO_EVERYONE",
		})
		if err != nil {
			s.repo.UpdatePostStatus(ctx, post.ID, "failed", "", err.Error()) //nolint:errcheck
			continue
		}
		s.repo.UpdatePostStatus(ctx, post.ID, "published", resp.TikTokVideoID, "") //nolint:errcheck
		log.Printf("[Scheduler] Published post %s → TikTok %s", post.ID, resp.TikTokVideoID)
	}
}
