package posts

import (
	"fmt"
	"math/rand"
	"strings"

	"github.com/Pathapongoo11/tiktok-affiliate-platform/api/models"
)

// SuggestCaptionResponse is the payload returned by POST /api/posts/suggest-caption.
type SuggestCaptionResponse struct {
	Caption  string   `json:"caption"`
	Hashtags []string `json:"hashtags"`
}

// GenerateCaption produces a Thai TikTok-style caption and hashtag set from
// a product's attributes. The output is ready to use in the Content Editor.
// No external LLM is required — the function uses Thai marketing templates
// that match the style proven on TikTok Shop Thailand.
func GenerateCaption(product *models.Product) SuggestCaptionResponse {
	caption := buildCaption(product)
	hashtags := buildHashtags(product)
	return SuggestCaptionResponse{
		Caption:  caption,
		Hashtags: hashtags,
	}
}

// ── Caption templates ────────────────────────────────────────────────────

type captionTemplate struct {
	hooks    []string
	bodies   []string
	closings []string
}

var categoryTemplates = map[string]captionTemplate{
	"Beauty": {
		hooks: []string{
			"ผิวสวยใส ไม่ต้องรอนาน! ✨",
			"สาวๆ ต้องมี! ของดีราคาคุ้มมาก 💕",
			"เปลี่ยนผิวได้ใน 7 วัน? ลองดูสิ!",
			"ขายดีที่สุดในอาทิตย์นี้ 🔥",
		},
		bodies: []string{
			"%s ราคาแค่ ฿%s เท่านั้น 😱 ดีจริงไม่โม้",
			"แนะนำ %s ของดีราคา ฿%s ใช้แล้วปัง!",
			"%s สูตรพิเศษ ราคา ฿%s คุ้มค่ามากๆ 💯",
		},
		closings: []string{
			"📦 กดสั่งได้เลยที่ตะกร้าข้างล่าง ส่งไวมาก!",
			"🛒 ลิงก์ตะกร้าข้างล่างเลยค่า ของมีจำกัด!",
			"💌 สั่งเลยนะ stock มีไม่เยอะ ไม่อยากพลาด!",
		},
	},
	"Health": {
		hooks: []string{
			"สุขภาพดีไม่มีขาย ถ้าอยากได้ต้องรีบ! 💪",
			"ดูแลตัวเองก่อนนะ ด้วย %s 🌿",
			"เพื่อสุขภาพที่ดีของคุณ ❤️",
			"ลองแล้วเปลี่ยนชีวิต! 🙌",
		},
		bodies: []string{
			"%s ราคาเพียง ฿%s บำรุงร่างกายจากภายใน 🌱",
			"แนะนำ %s สูตรเข้มข้น ฿%s ดีต่อสุขภาพมาก",
			"%s ของแท้ ราคา ฿%s วัตถุดิบคุณภาพสูง",
		},
		closings: []string{
			"📦 กดสั่งได้ที่ตะกร้าข้างล่าง ส่งด่วน!",
			"🛒 ซื้อง่ายๆ ผ่าน TikTok Shop เลยนะ",
			"💊 ลองดูสิ รับประกันว่าคุ้มค่า!",
		},
	},
	"Home": {
		hooks: []string{
			"บ้านสวยง่ายๆ ในงบไม่เกิน 300 บาท! 🏠",
			"อัพเกรดบ้านได้เลย กับสินค้าดีราคาถูก ✨",
			"ของใช้ในบ้านที่ต้องมี! 🏡",
		},
		bodies: []string{
			"%s เปลี่ยนบ้านได้ทันที ราคา ฿%s เท่านั้น",
			"แนะนำ %s สินค้าคุณภาพ ราคา ฿%s",
			"%s ใช้งานได้จริง ทนทาน ราคา ฿%s",
		},
		closings: []string{
			"🛒 กดสั่งได้เลยที่ตะกร้าข้างล่าง",
			"📦 ส่งฟรี! กดสั่งเลยนะ",
			"🏠 เปลี่ยนบ้านตอนนี้เลย ลิงก์ข้างล่าง",
		},
	},
	"Fashion": {
		hooks: []string{
			"แฟชั่นมาใหม่! ห้ามพลาด 👗",
			"เทรนด์ฮิตตอนนี้ คุณมีแล้วยัง? 💃",
			"ใส่ปุ๊บ ปังปั๊บ! ✨",
		},
		bodies: []string{
			"%s สวยมาก ราคา ฿%s เท่านั้น แมทช์ง่าย",
			"แนะนำ %s เทรนด์ใหม่ ราคา ฿%s",
			"%s ผ้าดี ใส่สบาย ราคา ฿%s ไม่แพงเลย",
		},
		closings: []string{
			"👗 กดสั่งได้เลยที่ตะกร้าข้างล่าง",
			"🛒 stock มีจำกัด รีบสั่งเลย!",
			"✨ ลิงก์ตะกร้าข้างล่างเลย ส่งไว!",
		},
	},
	"Food": {
		hooks: []string{
			"อร่อยมาก กินแล้วอยากกินอีก! 🍜",
			"ของกินแนะนำ รับรองว่าอร่อย 😋",
			"คนรักอาหารต้องลอง! 🍽️",
		},
		bodies: []string{
			"%s รสชาติเข้มข้น ราคา ฿%s อร่อยมาก",
			"แนะนำ %s ราคา ฿%s กินได้ทุกวัน",
			"%s สูตรพิเศษ ราคา ฿%s ต้องลอง!",
		},
		closings: []string{
			"🛒 กดสั่งได้เลย ส่งถึงบ้าน!",
			"🍜 ลิงก์ตะกร้าข้างล่างเลยค่า",
			"📦 ส่งฟรี! สั่งเลยนะ",
		},
	},
}

var defaultTemplate = captionTemplate{
	hooks: []string{
		"สินค้าดีมาแล้ว! ห้ามพลาด 🔥",
		"ของดีราคาคุ้ม ต้องมี! ✨",
		"แนะนำสินค้าขายดี 💯",
	},
	bodies: []string{
		"%s ราคาเพียง ฿%s ดีมากๆ",
		"แนะนำ %s ราคา ฿%s คุ้มค่า",
		"%s สินค้าคุณภาพ ราคา ฿%s",
	},
	closings: []string{
		"🛒 กดสั่งได้ที่ตะกร้าข้างล่างเลย!",
		"📦 ส่งไว รับประกันของแท้!",
		"💌 ลิงก์ข้างล่างเลยนะ ไม่ผิดหวัง!",
	},
}

func buildCaption(product *models.Product) string {
	tmpl, ok := categoryTemplates[product.Category]
	if !ok {
		tmpl = defaultTemplate
	}

	priceStr := formatPrice(product.Price)
	name := product.Name

	hook := tmpl.hooks[rand.Intn(len(tmpl.hooks))]
	bodyFmt := tmpl.bodies[rand.Intn(len(tmpl.bodies))]
	closing := tmpl.closings[rand.Intn(len(tmpl.closings))]

	// Insert product name/price if format verbs are present
	body := fmt.Sprintf(bodyFmt, name, priceStr)

	// Replace hook placeholder if present (some hooks use product name)
	hook = strings.ReplaceAll(hook, "%s", name)

	return hook + "\n\n" + body + "\n\n" + closing
}

func formatPrice(price float64) string {
	if price == float64(int(price)) {
		return fmt.Sprintf("%.0f", price)
	}
	return fmt.Sprintf("%.2f", price)
}

// ── Hashtag builder ──────────────────────────────────────────────────────

var categoryHashtags = map[string][]string{
	"Beauty": {
		"สกินแคร์", "ผิวสวย", "ความงาม", "skincare", "beauty",
		"tiktokshop", "ของดีราคาถูก", "beautyreview",
	},
	"Health": {
		"สุขภาพ", "อาหารเสริม", "สุขภาพดี", "health", "wellness",
		"tiktokshop", "ของดีราคาถูก", "healthtips",
	},
	"Home": {
		"ของแต่งบ้าน", "บ้านสวย", "homedecor", "ของใช้ในบ้าน",
		"tiktokshop", "ของดีราคาถูก", "homelife",
	},
	"Fashion": {
		"แฟชั่น", "fashion", "ootd", "สไตล์", "เสื้อผ้า",
		"tiktokshop", "ของดีราคาถูก", "fashionista",
	},
	"Food": {
		"อาหาร", "กินอะไรดี", "foodie", "อร่อย", "food",
		"tiktokshop", "ของกินแนะนำ", "foodreview",
	},
	"Pet": {
		"สัตว์เลี้ยง", "หมาแมว", "pet", "petlover", "สุนัข",
		"tiktokshop", "ของดีราคาถูก",
	},
}

var baseHashtags = []string{
	"tiktokshopthailand", "ขายของออนไลน์", "แนะนำสินค้า",
	"tiktokmademebuyit", "สินค้าแนะนำ",
}

func buildHashtags(product *models.Product) []string {
	tags := make([]string, 0, 12)

	// Category-specific tags
	if catTags, ok := categoryHashtags[product.Category]; ok {
		tags = append(tags, catTags...)
	}

	// Base affiliate tags
	tags = append(tags, baseHashtags...)

	// Product name as tag (sanitised: remove spaces, lowercase)
	nameParts := strings.Fields(product.Name)
	if len(nameParts) > 0 && len(nameParts) <= 3 {
		tags = append(tags, strings.Join(nameParts, ""))
	}

	// Commission rate hint
	if product.CommissionRate >= 10 && product.CommissionRate <= 20 {
		tags = append(tags, "คอมดี", "สินค้าคอมสูง")
	}

	// Deduplicate while preserving order
	seen := make(map[string]bool)
	unique := tags[:0]
	for _, t := range tags {
		if !seen[t] {
			seen[t] = true
			unique = append(unique, t)
		}
	}

	// Cap at 15 hashtags (TikTok sweet spot)
	if len(unique) > 15 {
		unique = unique[:15]
	}
	return unique
}
