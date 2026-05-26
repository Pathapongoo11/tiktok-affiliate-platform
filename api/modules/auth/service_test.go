package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/crypto/bcrypt"

	"github.com/Pathapongoo11/tiktok-affiliate-platform/api/models"
	"github.com/Pathapongoo11/tiktok-affiliate-platform/api/modules/auth"
)

// --- Mock Repository ---

type mockUserRepo struct {
	mock.Mock
}

func (m *mockUserRepo) CreateUser(ctx context.Context, user *models.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *mockUserRepo) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *mockUserRepo) GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

const jwtSecret = "test_secret_key"

// --- Register Tests ---

func TestRegister_Success(t *testing.T) {
	repo := new(mockUserRepo)
	svc := auth.NewService(repo, jwtSecret)

	repo.On("GetUserByEmail", mock.Anything, "alice@example.com").Return(nil, nil)
	repo.On("CreateUser", mock.Anything, mock.AnythingOfType("*models.User")).Return(nil)

	req := models.RegisterRequest{
		Email:       "alice@example.com",
		Password:    "password123",
		DisplayName: "Alice",
	}

	resp, err := svc.Register(context.Background(), req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.Token)
	assert.Equal(t, "alice@example.com", resp.User.Email)
	assert.Equal(t, "Alice", resp.User.DisplayName)
	// Password hash should not equal plaintext
	assert.NotEqual(t, "password123", resp.User.PasswordHash)
	// Verify bcrypt hash
	err = bcrypt.CompareHashAndPassword([]byte(resp.User.PasswordHash), []byte("password123"))
	assert.NoError(t, err, "stored password hash must match plaintext")
	repo.AssertExpectations(t)
}

func TestRegister_DuplicateEmail(t *testing.T) {
	repo := new(mockUserRepo)
	svc := auth.NewService(repo, jwtSecret)

	existingUser := &models.User{
		ID:    uuid.New(),
		Email: "bob@example.com",
	}
	repo.On("GetUserByEmail", mock.Anything, "bob@example.com").Return(existingUser, nil)

	req := models.RegisterRequest{
		Email:    "bob@example.com",
		Password: "password123",
	}

	resp, err := svc.Register(context.Background(), req)

	assert.Nil(t, resp)
	assert.ErrorIs(t, err, auth.ErrEmailTaken)
	repo.AssertExpectations(t)
}

func TestRegister_RepoError(t *testing.T) {
	repo := new(mockUserRepo)
	svc := auth.NewService(repo, jwtSecret)

	repo.On("GetUserByEmail", mock.Anything, "err@example.com").Return(nil, errors.New("db error"))

	req := models.RegisterRequest{Email: "err@example.com", Password: "pass"}
	resp, err := svc.Register(context.Background(), req)

	assert.Nil(t, resp)
	assert.Error(t, err)
	repo.AssertExpectations(t)
}

// --- Login Tests ---

func TestLogin_Success(t *testing.T) {
	repo := new(mockUserRepo)
	svc := auth.NewService(repo, jwtSecret)

	hash, _ := bcrypt.GenerateFromPassword([]byte("securepass"), bcrypt.DefaultCost)
	user := &models.User{
		ID:           uuid.New(),
		Email:        "carol@example.com",
		PasswordHash: string(hash),
		DisplayName:  "Carol",
	}
	repo.On("GetUserByEmail", mock.Anything, "carol@example.com").Return(user, nil)

	req := models.LoginRequest{
		Email:    "carol@example.com",
		Password: "securepass",
	}

	resp, err := svc.Login(context.Background(), req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.Token)
	assert.Equal(t, user.ID, resp.User.ID)
	repo.AssertExpectations(t)
}

func TestLogin_WrongPassword(t *testing.T) {
	repo := new(mockUserRepo)
	svc := auth.NewService(repo, jwtSecret)

	hash, _ := bcrypt.GenerateFromPassword([]byte("correct_pass"), bcrypt.DefaultCost)
	user := &models.User{
		ID:           uuid.New(),
		Email:        "dave@example.com",
		PasswordHash: string(hash),
	}
	repo.On("GetUserByEmail", mock.Anything, "dave@example.com").Return(user, nil)

	req := models.LoginRequest{
		Email:    "dave@example.com",
		Password: "wrong_pass",
	}

	resp, err := svc.Login(context.Background(), req)

	assert.Nil(t, resp)
	assert.ErrorIs(t, err, auth.ErrInvalidCreds)
	repo.AssertExpectations(t)
}

func TestLogin_UserNotFound(t *testing.T) {
	repo := new(mockUserRepo)
	svc := auth.NewService(repo, jwtSecret)

	repo.On("GetUserByEmail", mock.Anything, "ghost@example.com").Return(nil, nil)

	req := models.LoginRequest{
		Email:    "ghost@example.com",
		Password: "anypass",
	}

	resp, err := svc.Login(context.Background(), req)

	assert.Nil(t, resp)
	assert.ErrorIs(t, err, auth.ErrInvalidCreds)
	repo.AssertExpectations(t)
}

// --- JWT Token Tests ---

func TestGenerateToken_ContainsCorrectClaims(t *testing.T) {
	repo := new(mockUserRepo)
	svc := auth.NewService(repo, jwtSecret)

	hash, _ := bcrypt.GenerateFromPassword([]byte("pass"), bcrypt.DefaultCost)
	userID := uuid.New()
	user := &models.User{
		ID:           userID,
		Email:        "tok@example.com",
		PasswordHash: string(hash),
	}
	repo.On("GetUserByEmail", mock.Anything, "tok@example.com").Return(user, nil)

	resp, err := svc.Login(context.Background(), models.LoginRequest{
		Email:    "tok@example.com",
		Password: "pass",
	})

	assert.NoError(t, err)
	assert.NotEmpty(t, resp.Token)

	// Parse and inspect claims
	tok, err := jwt.Parse(resp.Token, func(t *jwt.Token) (interface{}, error) {
		return []byte(jwtSecret), nil
	})
	assert.NoError(t, err)
	assert.True(t, tok.Valid)

	claims, ok := tok.Claims.(jwt.MapClaims)
	assert.True(t, ok)
	assert.Equal(t, userID.String(), claims["user_id"].(string))

	expUnix := int64(claims["exp"].(float64))
	assert.Greater(t, expUnix, time.Now().Unix())
}

func TestValidateToken_Valid(t *testing.T) {
	repo := new(mockUserRepo)
	svc := auth.NewService(repo, jwtSecret)

	hash, _ := bcrypt.GenerateFromPassword([]byte("pass123"), bcrypt.DefaultCost)
	userID := uuid.New()
	user := &models.User{
		ID:           userID,
		Email:        "valid@example.com",
		PasswordHash: string(hash),
	}
	repo.On("GetUserByEmail", mock.Anything, "valid@example.com").Return(user, nil)

	resp, err := svc.Login(context.Background(), models.LoginRequest{
		Email:    "valid@example.com",
		Password: "pass123",
	})
	assert.NoError(t, err)

	// Token should parse correctly
	tok, err := jwt.Parse(resp.Token, func(t *jwt.Token) (interface{}, error) {
		return []byte(jwtSecret), nil
	})
	assert.NoError(t, err)
	assert.True(t, tok.Valid)
}

func TestValidateToken_Expired(t *testing.T) {
	// Create a token that is already expired
	claims := jwt.MapClaims{
		"user_id": uuid.New().String(),
		"exp":     time.Now().Add(-1 * time.Hour).Unix(),
		"iat":     time.Now().Add(-2 * time.Hour).Unix(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, _ := tok.SignedString([]byte(jwtSecret))

	_, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		return []byte(jwtSecret), nil
	})

	assert.Error(t, err, "expired token should fail validation")
}

// --- GetCurrentUser Tests ---

func TestGetCurrentUser_Success(t *testing.T) {
	repo := new(mockUserRepo)
	svc := auth.NewService(repo, jwtSecret)

	userID := uuid.New()
	user := &models.User{ID: userID, Email: "me@example.com"}
	repo.On("GetUserByID", mock.Anything, userID).Return(user, nil)

	result, err := svc.GetCurrentUser(context.Background(), userID)

	assert.NoError(t, err)
	assert.Equal(t, userID, result.ID)
	repo.AssertExpectations(t)
}

func TestGetCurrentUser_NotFound(t *testing.T) {
	repo := new(mockUserRepo)
	svc := auth.NewService(repo, jwtSecret)

	userID := uuid.New()
	repo.On("GetUserByID", mock.Anything, userID).Return(nil, nil)

	result, err := svc.GetCurrentUser(context.Background(), userID)

	assert.Nil(t, result)
	assert.ErrorIs(t, err, auth.ErrUserNotFound)
	repo.AssertExpectations(t)
}
