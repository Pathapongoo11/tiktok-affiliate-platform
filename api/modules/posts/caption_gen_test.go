package posts_test

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/Pathapongoo11/tiktok-affiliate-platform/api/models"
	"github.com/Pathapongoo11/tiktok-affiliate-platform/api/modules/posts"
)

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

func makeProduct(category string, name string, price float64, commission float64) *models.Product {
	return &models.Product{
		ID:             uuid.New(),
		Name:           name,
		Category:       category,
		Price:          price,
		CommissionRate: commission,
	}
}

// ---------------------------------------------------------------------------
// GenerateCaption — caption tests
// ---------------------------------------------------------------------------

func TestGenerateCaption_ContainsProductName(t *testing.T) {
	product := makeProduct("Beauty", "Vitamin C Serum", 299, 15)
	result := posts.GenerateCaption(product)

	assert.Contains(t, result.Caption, "Vitamin C Serum",
		"caption must include the product name")
}

func TestGenerateCaption_ContainsPrice(t *testing.T) {
	product := makeProduct("Health", "Collagen Drink", 450, 12)
	result := posts.GenerateCaption(product)

	assert.Contains(t, result.Caption, "450",
		"caption must include the formatted price")
}

func TestGenerateCaption_ThreeParts(t *testing.T) {
	// buildCaption returns hook + "\n\n" + body + "\n\n" + closing — 2 blank-line separators
	product := makeProduct("Fashion", "Floral Dress", 599, 10)
	result := posts.GenerateCaption(product)

	parts := strings.Split(result.Caption, "\n\n")
	assert.GreaterOrEqual(t, len(parts), 3,
		"caption must have at least 3 sections separated by blank lines")
}

func TestGenerateCaption_NonEmptyCaption(t *testing.T) {
	for _, cat := range []string{"Beauty", "Health", "Home", "Fashion", "Food", "Unknown"} {
		p := makeProduct(cat, "Test Product", 199, 10)
		result := posts.GenerateCaption(p)
		assert.NotEmpty(t, result.Caption, "caption must not be empty for category %q", cat)
	}
}

func TestGenerateCaption_DefaultCategoryFallback(t *testing.T) {
	product := makeProduct("Electronics", "Smart Watch", 1999, 8)
	result := posts.GenerateCaption(product)

	// Default template must still produce a caption with the product name
	assert.Contains(t, result.Caption, "Smart Watch")
	assert.NotEmpty(t, result.Hashtags)
}

func TestGenerateCaption_PriceFormatsInteger(t *testing.T) {
	// Price 300.00 → "300" (no decimal)
	product := makeProduct("Home", "Lamp", 300.0, 10)
	result := posts.GenerateCaption(product)

	assert.Contains(t, result.Caption, "300")
	assert.NotContains(t, result.Caption, "300.00")
}

func TestGenerateCaption_PriceFormatsDecimal(t *testing.T) {
	// Price 199.50 → "199.50"
	product := makeProduct("Food", "Special Sauce", 199.50, 10)
	result := posts.GenerateCaption(product)

	assert.Contains(t, result.Caption, "199.50")
}

// ---------------------------------------------------------------------------
// GenerateCaption — hashtag tests
// ---------------------------------------------------------------------------

func TestGenerateCaption_HasHashtags(t *testing.T) {
	product := makeProduct("Beauty", "Moisturiser", 399, 10)
	result := posts.GenerateCaption(product)

	assert.NotEmpty(t, result.Hashtags, "hashtags must not be empty")
}

func TestGenerateCaption_MaxFifteenHashtags(t *testing.T) {
	product := makeProduct("Beauty", "Face Wash", 149, 5)
	result := posts.GenerateCaption(product)

	assert.LessOrEqual(t, len(result.Hashtags), 15, "TikTok cap: max 15 hashtags")
}

func TestGenerateCaption_ContainsBaseHashtags(t *testing.T) {
	product := makeProduct("Health", "Omega 3", 299, 10)
	result := posts.GenerateCaption(product)

	// baseHashtags always appended
	assert.Contains(t, result.Hashtags, "tiktokshopthailand")
	assert.Contains(t, result.Hashtags, "สินค้าแนะนำ")
}

func TestGenerateCaption_CategoryHashtagsIncluded(t *testing.T) {
	cases := map[string]string{
		"Beauty":  "สกินแคร์",
		"Health":  "สุขภาพ",
		"Home":    "ของแต่งบ้าน",
		"Fashion": "แฟชั่น",
		"Food":    "อาหาร",
	}
	for cat, expectedTag := range cases {
		p := makeProduct(cat, "Item", 100, 10)
		result := posts.GenerateCaption(p)
		assert.Contains(t, result.Hashtags, expectedTag,
			"category %q should include tag %q", cat, expectedTag)
	}
}

func TestGenerateCaption_HighCommissionTags(t *testing.T) {
	// CommissionRate 10–20 → adds commission tags. Use a category with fewer base
	// tags (Pet has fewer) so both can fit within the 15-tag cap.
	product := makeProduct("Pet", "Dog Food", 299, 15)
	result := posts.GenerateCaption(product)

	// At least one of the two commission tags must be present
	hasTag := false
	for _, tag := range result.Hashtags {
		if tag == "คอมดี" || tag == "สินค้าคอมสูง" {
			hasTag = true
			break
		}
	}
	assert.True(t, hasTag, "high-commission product should include a commission hashtag")
}

func TestGenerateCaption_LowCommissionNoSpecialTags(t *testing.T) {
	// CommissionRate < 10 → no "คอมดี"
	product := makeProduct("Beauty", "Toner", 199, 5)
	result := posts.GenerateCaption(product)

	assert.NotContains(t, result.Hashtags, "คอมดี")
}

func TestGenerateCaption_NoDuplicateHashtags(t *testing.T) {
	product := makeProduct("Beauty", "Serum", 499, 18)
	result := posts.GenerateCaption(product)

	seen := make(map[string]bool)
	for _, tag := range result.Hashtags {
		assert.False(t, seen[tag], "duplicate hashtag found: %q", tag)
		seen[tag] = true
	}
}

func TestGenerateCaption_ShortProductNameAsTag(t *testing.T) {
	// Single-word product name should appear as a hashtag
	product := makeProduct("Food", "Kimchi", 89, 8)
	result := posts.GenerateCaption(product)

	assert.Contains(t, result.Hashtags, "Kimchi")
}

func TestGenerateCaption_LongProductNameNotTag(t *testing.T) {
	// Product names > 3 words are skipped as hashtags
	product := makeProduct("Home", "Very Long Product Name Here Extra", 299, 10)
	result := posts.GenerateCaption(product)

	assert.NotContains(t, result.Hashtags, "VeryLongProductNameHereExtra")
}

// ---------------------------------------------------------------------------
// SuggestCaptionResponse — struct shape
// ---------------------------------------------------------------------------

func TestSuggestCaptionResponse_Fields(t *testing.T) {
	product := makeProduct("Fashion", "Crop Top", 350, 12)
	result := posts.GenerateCaption(product)

	assert.IsType(t, "", result.Caption)
	assert.IsType(t, []string{}, result.Hashtags)
}
