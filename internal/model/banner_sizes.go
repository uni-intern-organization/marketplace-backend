package model

import (
	"fmt"
	"math"
)

const BannerDimensionTolerance = 0.05
const MaxBannerFileBytes = 10 << 20 // 10 MB — high-res source images (e.g. 9465×1171 JPEG)
const MinBannerAbsWidth = 200
const MinBannerAbsHeight = 50

type BannerSize struct {
	Width  int
	Height int
}

// BannerAllowedSizes returns primary DB size plus optional alternates per placement.
func BannerAllowedSizes(code string, primaryW, primaryH int) []BannerSize {
	sizes := []BannerSize{{Width: primaryW, Height: primaryH}}
	switch code {
	case "home_hero":
		sizes = append(sizes, BannerSize{Width: 640, Height: 360}) // 16:9
	case "jobs_inline":
		sizes = append(sizes, BannerSize{Width: 336, Height: 280})
	case "hackathons_top":
		sizes = append(sizes, BannerSize{Width: 728, Height: 90}) // leaderboard
	}
	return sizes
}

func bannerDimensionsOK(actualW, actualH, expectW, expectH int) bool {
	const tolerance = BannerDimensionTolerance
	minW := float64(expectW) * (1 - tolerance)
	maxW := float64(expectW) * (1 + tolerance)
	minH := float64(expectH) * (1 - tolerance)
	maxH := float64(expectH) * (1 + tolerance)
	w := float64(actualW)
	h := float64(actualH)
	return w >= minW && w <= maxW && h >= minH && h <= maxH
}

func bannerAspectRatioOK(actualW, actualH, expectW, expectH int) bool {
	if expectW <= 0 || expectH <= 0 || actualW <= 0 || actualH <= 0 {
		return false
	}
	if actualW < MinBannerAbsWidth || actualH < MinBannerAbsHeight {
		return false
	}
	expected := float64(expectW) / float64(expectH)
	actual := float64(actualW) / float64(actualH)
	return math.Abs(actual-expected)/expected <= BannerDimensionTolerance
}

// BannerDimensionsMatch returns true if size matches exactly (±5%) or same aspect ratio with sufficient resolution.
func BannerDimensionsMatch(actualW, actualH int, allowed []BannerSize) bool {
	for _, s := range allowed {
		if bannerDimensionsOK(actualW, actualH, s.Width, s.Height) {
			return true
		}
		if bannerAspectRatioOK(actualW, actualH, s.Width, s.Height) {
			return true
		}
	}
	return false
}

func BannerSizeRangeLabel(w, h int) string {
	minW := int(float64(w) * (1 - BannerDimensionTolerance))
	maxW := int(float64(w) * (1 + BannerDimensionTolerance))
	minH := int(float64(h) * (1 - BannerDimensionTolerance))
	maxH := int(float64(h) * (1 + BannerDimensionTolerance))
	if minW == maxW && minH == maxH {
		return fmt.Sprintf("%d×%d", w, h)
	}
	return fmt.Sprintf("%d×%d (допустимо %d–%d × %d–%d)", w, h, minW, maxW, minH, maxH)
}

func BannerAllowedSizesLabel(allowed []BannerSize) string {
	if len(allowed) == 0 {
		return ""
	}
	out := BannerSizeRangeLabel(allowed[0].Width, allowed[0].Height)
	for i := 1; i < len(allowed); i++ {
		out += " или " + BannerSizeRangeLabel(allowed[i].Width, allowed[i].Height)
	}
	return out
}

func BannerDimensionError(actualW, actualH int, allowed []BannerSize) string {
	return fmt.Sprintf(
		"Размер изображения %d×%d px не подходит. Рекомендуемый размер: %s. Допускается то же соотношение сторон (±5%%) при разрешении не ниже рекомендуемого, в т.ч. высокое (JPEG, PNG, WEBP, до 10 МБ).",
		actualW, actualH, BannerAllowedSizesLabel(allowed),
	)
}
