package report

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const cninfoSearchURL = "http://www.cninfo.com.cn/new/fulltextSearch/full"
const cninfoDownloadBase = "http://static.cninfo.com.cn/"

type CninfoAnnouncement struct {
	ID          string `json:"announcementId"`
	Title       string `json:"announcementTitle"`
	Time        int64  `json:"announcementTime"`
	AdjunctURL  string `json:"adjunctUrl"`
	AdjunctSize int    `json:"adjunctSize"`
	SecCode     string `json:"secCode"`
	SecName     string `json:"secName"`
}

type CninfoReport struct {
	Year        int    `json:"year"`
	Title       string `json:"title"`
	Date        string `json:"date"`
	DownloadURL string `json:"download_url"`
	SizeMB      string `json:"size_mb"`
}

var yearPattern = regexp.MustCompile(`(20[012]\d)\s*年`)

func stockNameFromCode(tsCode string, fallback string) string {
	if fallback != "" {
		return fallback
	}
	parts := strings.SplitN(tsCode, ".", 2)
	return parts[0]
}

func SearchCninfoReports(tsCode, stockName string) ([]CninfoReport, error) {
	name := stockNameFromCode(tsCode, stockName)
	searchKey := name + " 年度报告"

	form := url.Values{}
	form.Set("searchkey", searchKey)
	form.Set("sDate", "")
	form.Set("eDate", "")
	form.Set("isfulltext", "false")
	form.Set("sortName", "pubdate")
	form.Set("sortType", "desc")
	form.Set("pageNum", "1")
	form.Set("pageSize", "50")

	client := &http.Client{Timeout: 20 * time.Second}
	req, err := http.NewRequest("POST", cninfoSearchURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cninfo request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("cninfo read body: %w", err)
	}

	var result struct {
		Announcements     []CninfoAnnouncement `json:"announcements"`
		TotalAnnouncement int                  `json:"totalAnnouncement"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("cninfo parse: %w (body: %.200s)", err, string(body))
	}

	plainCode := strings.SplitN(tsCode, ".", 2)[0]

	var reports []CninfoReport
	seen := map[int]bool{}
	for _, ann := range result.Announcements {
		if ann.SecCode != plainCode {
			continue
		}

		title := strings.ReplaceAll(ann.Title, "<em>", "")
		title = strings.ReplaceAll(title, "</em>", "")

		if !isAnnualReport(title) {
			continue
		}

		year := extractYear(title, ann.Time)
		if year == 0 || seen[year] {
			continue
		}
		seen[year] = true

		sizeMB := fmt.Sprintf("%.1f", float64(ann.AdjunctSize)/1024)
		reports = append(reports, CninfoReport{
			Year:        year,
			Title:       title,
			Date:        time.UnixMilli(ann.Time).Format("2006-01-02"),
			DownloadURL: cninfoDownloadBase + ann.AdjunctURL,
			SizeMB:      sizeMB,
		})
	}

	sort.Slice(reports, func(i, j int) bool {
		return reports[i].Year > reports[j].Year
	})
	return reports, nil
}

func isAnnualReport(title string) bool {
	excludes := []string{
		"摘要", "补充", "更正", "英文", "审计", "内部控制", "社会责任",
		"可持续", "annual", "决议", "董事会", "监事会", "半年",
		"季度", "意见", "鉴证", "评估",
	}
	lower := strings.ToLower(title)
	for _, ex := range excludes {
		if strings.Contains(lower, ex) {
			return false
		}
	}
	return strings.Contains(title, "年度报告") || strings.Contains(title, "年报")
}

func extractYear(title string, timeMs int64) int {
	m := yearPattern.FindStringSubmatch(title)
	if len(m) >= 2 {
		y, _ := strconv.Atoi(m[1])
		if y >= 2000 && y <= 2030 {
			return y
		}
	}
	t := time.UnixMilli(timeMs)
	return t.Year() - 1
}

// CninfoNewsItem represents one announcement for the news feed.
type CninfoNewsItem struct {
	Title   string `json:"title"`
	Date    string `json:"date"`
	URL     string `json:"url"`
	Type    string `json:"type"`
	SizeMB  string `json:"size_mb,omitempty"`
}

const cninfoAnnouncementPageBase = "http://www.cninfo.com.cn/new/disclosure/detail?stockCode=%s&announcementId=%s&orgId=%s"

// SearchCninfoNews queries recent announcements for a stock (all types).
func SearchCninfoNews(tsCode, stockName string, pageSize int) ([]CninfoNewsItem, error) {
	name := stockNameFromCode(tsCode, stockName)
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 30
	}

	form := url.Values{}
	form.Set("searchkey", name)
	form.Set("sDate", "")
	form.Set("eDate", "")
	form.Set("isfulltext", "false")
	form.Set("sortName", "pubdate")
	form.Set("sortType", "desc")
	form.Set("pageNum", "1")
	form.Set("pageSize", strconv.Itoa(pageSize))

	client := &http.Client{Timeout: 20 * time.Second}
	req, err := http.NewRequest("POST", cninfoSearchURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cninfo news request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("cninfo news read: %w", err)
	}

	var result struct {
		Announcements []struct {
			ID         string `json:"announcementId"`
			Title      string `json:"announcementTitle"`
			Time       int64  `json:"announcementTime"`
			AdjunctURL string `json:"adjunctUrl"`
			AdjunctSize int   `json:"adjunctSize"`
			SecCode    string `json:"secCode"`
			OrgId      string `json:"orgId"`
		} `json:"announcements"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("cninfo news parse: %w", err)
	}

	plainCode := strings.SplitN(tsCode, ".", 2)[0]
	var items []CninfoNewsItem
	for _, ann := range result.Announcements {
		if ann.SecCode != plainCode {
			continue
		}
		title := strings.ReplaceAll(ann.Title, "<em>", "")
		title = strings.ReplaceAll(title, "</em>", "")

		annoURL := fmt.Sprintf(cninfoAnnouncementPageBase, ann.SecCode, ann.ID, ann.OrgId)
		pdfURL := cninfoDownloadBase + ann.AdjunctURL

		typeName := classifyAnnouncement(title)

		item := CninfoNewsItem{
			Title:  title,
			Date:   time.UnixMilli(ann.Time).Format("2006-01-02"),
			URL:    annoURL,
			Type:   typeName,
		}
		if ann.AdjunctSize > 0 {
			item.SizeMB = fmt.Sprintf("%.1f", float64(ann.AdjunctSize)/1024)
		}
		_ = pdfURL
		items = append(items, item)
	}
	return items, nil
}

func classifyAnnouncement(title string) string {
	switch {
	case strings.Contains(title, "年度报告") || strings.Contains(title, "年报"):
		return "年报"
	case strings.Contains(title, "半年度") || strings.Contains(title, "半年报"):
		return "半年报"
	case strings.Contains(title, "季度报告") || strings.Contains(title, "季报"):
		return "季报"
	case strings.Contains(title, "分红") || strings.Contains(title, "派息"):
		return "分红"
	case strings.Contains(title, "增持") || strings.Contains(title, "减持"):
		return "增减持"
	case strings.Contains(title, "回购"):
		return "回购"
	case strings.Contains(title, "董事会") || strings.Contains(title, "监事会") || strings.Contains(title, "股东大会"):
		return "会议"
	case strings.Contains(title, "关联交易"):
		return "关联交易"
	default:
		return "公告"
	}
}

func DownloadCninfoPDF(downloadURL, savePath string) error {
	client := &http.Client{Timeout: 120 * time.Second}
	req, err := http.NewRequest("GET", downloadURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "http://www.cninfo.com.cn/")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("download HTTP %d", resp.StatusCode)
	}

	f, err := os.Create(savePath)
	if err != nil {
		return err
	}
	defer f.Close()

	written, err := io.Copy(f, resp.Body)
	if err != nil {
		return fmt.Errorf("write failed: %w", err)
	}
	log.Printf("cninfo: downloaded %.1f MB to %s", float64(written)/(1024*1024), savePath)
	return nil
}
