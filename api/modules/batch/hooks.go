package batch

import (
	"fmt"
	"math/rand"
	"strings"

	"github.com/Pathapongoo11/tiktok-affiliate-platform/api/models"
)

// bestPostTimes are the strongest Thai TikTok posting slots (ICT), matching the
// suggestions already used in the Content Editor. PostsPerDay picks from the front.
var bestPostTimes = []struct{ Hour, Min int }{
	{19, 0}, // prime evening
	{12, 0}, // lunch
	{21, 0}, // late evening
	{7, 0},  // morning commute
}

// hookTemplates are scroll-stopping 3-second openers by category (Thai).
// %s is replaced with the product name when present.
var hookTemplates = map[string][]string{
	"Beauty": {
		"หยุดเลื่อน! ผิวสวยใน 7 วันมีจริง ✨",
		"ใครยังหาครีมดีๆ ไม่เจอ ดูคลิปนี้!",
		"ของมันต้องมี! %s ที่สาวๆ แย่งกันซื้อ 🔥",
		"เคล็ดลับผิวใส ที่บิวตี้บล็อกเกอร์ไม่บอก 🤫",
	},
	"Health": {
		"ตื่นแล้วยังเหนื่อย? ไม่ใช่เรื่องอายุ 😱",
		"กินตัวนี้ทุกวัน ร่างกายเปลี่ยนไปเลย 💪",
		"หมอไม่บอก แต่ฉันจะบอก! %s ดียังไง 🌿",
		"สุขภาพดีเริ่มได้วันนี้ ด้วย %s",
	},
	"Fashion": {
		"แต่งตัวยังไงให้ดูแพง? เริ่มจากตัวนี้ 👗",
		"ไอเทมนี้ใส่แล้วปังทุกชุด หยุดดูก่อน!",
		"%s ที่ทุกคนถามว่าซื้อที่ไหน 💕",
		"ลุคนี้งบไม่ถึงร้อย เชื่อไหม? 😮",
	},
	"Electronics": {
		"แกดเจ็ตตัวนี้เปลี่ยนชีวิตเลย ดูสิ! ⚡",
		"ของมันเทพ! %s ที่คุ้มเกินราคา",
		"อย่าเพิ่งซื้อ ถ้ายังไม่ดูคลิปนี้ 🛑",
		"เทคโนโลยีใหม่ ราคาเบาๆ ต้องมี!",
	},
	"Food": {
		"กินเล่นได้ทั้งวัน อร่อยจนหยุดไม่ได้ 😋",
		"ของกินตัวนี้ขายดีจนของหมดตลอด!",
		"%s ที่สายกินห้ามพลาด เด็ดมาก 🔥",
		"หิวไหม? ตัวนี้แก้หิวได้ดีงาม",
	},
}

var defaultHooks = []string{
	"หยุดเลื่อน! ของดีบอกต่อมาแล้ว 🔥",
	"ตัวนี้แหละที่ทุกคนตามหา! ดูเลย",
	"%s ที่ขายดีจนต้องสั่งเพิ่ม 😱",
	"ของมันต้องมี ราคาคุ้มเกินคาด!",
}

// GenerateHook returns a scroll-stopping Thai opener for a product.
func GenerateHook(p *models.Product) string {
	hooks, ok := hookTemplates[p.Category]
	if !ok {
		hooks = defaultHooks
	}
	h := hooks[rand.Intn(len(hooks))]
	// Only format when the template actually has a %s placeholder, otherwise
	// fmt.Sprintf appends "%!(EXTRA string=...)".
	if strings.Contains(h, "%s") {
		return fmt.Sprintf(h, p.Name)
	}
	return h
}

// GenerateScript returns a short voice/caption script (Thai) for a product —
// usable directly as ai_talking TTS text. It chains hook → value → call to action.
func GenerateScript(p *models.Product) string {
	hook := GenerateHook(p)
	price := formatPrice(p.Price)
	value := fmt.Sprintf("%s ราคาแค่ %s บาท คุณภาพดีเกินราคา", p.Name, price)
	if p.Description != "" {
		value = fmt.Sprintf("%s %s ราคาแค่ %s บาท", p.Description, p.Name, price)
	}
	cta := "กดสั่งที่ตะกร้าด้านล่างได้เลยนะคะ ของมีจำนวนจำกัด"
	return hook + " " + value + " " + cta
}

// formatPrice renders a price without trailing .00.
func formatPrice(price float64) string {
	if price == float64(int(price)) {
		return fmt.Sprintf("%d", int(price))
	}
	return fmt.Sprintf("%.2f", price)
}
