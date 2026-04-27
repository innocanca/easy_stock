package live

import (
	"encoding/base64"
	"strings"
)

// SectorIDFromIndustry 把申万/行业中文名编成 URL 安全的板块 id。
func SectorIDFromIndustry(industry string) string {
	if industry == "" || industry == "—" {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString([]byte(industry))
}

// IndustryFromSectorID 解码板块 id。
func IndustryFromSectorID(id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", ErrSectorNotFound
	}
	// RawURLEncoding 解码时补齐 padding
	for len(id)%4 != 0 {
		id += "="
	}
	b, err := base64.RawURLEncoding.DecodeString(id)
	if err != nil {
		return "", ErrSectorNotFound
	}
	s := strings.TrimSpace(string(b))
	if s == "" {
		return "", ErrSectorNotFound
	}
	return s, nil
}
