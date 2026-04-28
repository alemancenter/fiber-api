package services

import (
	"sync"
	"time"

	"github.com/alemancenter/fiber-api/internal/database"
	"github.com/alemancenter/fiber-api/internal/repositories"
	"github.com/gofiber/fiber/v2"
)

type AnalyticsService interface {
	GetVisitorAnalytics(dbCode database.CountryID, days int) fiber.Map
	PruneAnalytics(dbCode database.CountryID, days int) int64
	GetDashboardSummary(dbCode database.CountryID) fiber.Map
	GetContentAnalytics(dbCode database.CountryID) fiber.Map
}

type analyticsService struct {
	repo repositories.AnalyticsRepository
}

func NewAnalyticsService(repo repositories.AnalyticsRepository) AnalyticsService {
	return &analyticsService{repo: repo}
}

func (s *analyticsService) GetVisitorAnalytics(dbCode database.CountryID, days int) fiber.Map {
	now := time.Now()
	since := now.AddDate(0, 0, -days)
	prevSince := now.AddDate(0, 0, -days*2)
	activeWindow := now.Add(-15 * time.Minute)
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	yesterdayStart := todayStart.AddDate(0, 0, -1)

	var wg sync.WaitGroup

	var currentActive, currentMembers, currentGuests, totalToday, totalYesterday int64
	var activeRows []repositories.ActiveRow

	var totalUsers, activeUsers, newToday int64
	var countryStats []repositories.CountryRow
	var dailyRows []repositories.DailyRow
	var deviceRows []repositories.DeviceRow
	var refRows []repositories.RefRow
	var prevTotalVisits int64

	wg.Add(6)

	// visitor_stats
	go func() {
		defer wg.Done()
		currentActive, currentMembers, currentGuests, totalToday, totalYesterday = s.repo.GetVisitorStats(dbCode, activeWindow, todayStart, yesterdayStart)
		activeRows, _ = s.repo.GetActiveVisitors(dbCode, activeWindow)
	}()

	// user_stats
	go func() {
		defer wg.Done()
		totalUsers, activeUsers, newToday = s.repo.GetUserStats(todayStart, now.AddDate(0, 0, -30))
	}()

	// country_stats
	go func() {
		defer wg.Done()
		countryStats, _ = s.repo.GetCountryStats(dbCode, since)
	}()

	// chart_data
	go func() {
		defer wg.Done()
		dailyRows, _ = s.repo.GetDailyChartData(dbCode, since)
	}()

	// device_stats
	go func() {
		defer wg.Done()
		deviceRows, _ = s.repo.GetDeviceStats(dbCode, since)
	}()

	// traffic_sources
	go func() {
		defer wg.Done()
		refRows, _ = s.repo.GetTrafficSources(dbCode, since)
		prevTotalVisits = s.repo.GetPrevTotalVisits(dbCode, prevSince, since)
	}()

	wg.Wait()

	// ---- assemble visitor_stats ----
	changeVal := 0.0
	if totalYesterday > 0 {
		changeVal = float64(totalToday-totalYesterday) / float64(totalYesterday) * 100
	}

	activeVisitors := make([]fiber.Map, 0, len(activeRows))
	for _, r := range activeRows {
		av := fiber.Map{
			"ip":               r.IPAddress,
			"country":          strVal(r.Country),
			"city":             strVal(r.City),
			"browser":          strVal(r.Browser),
			"os":               strVal(r.OS),
			"user_agent":       r.UserAgent,
			"current_page":     strVal(r.URL),
			"current_page_full": strVal(r.URL),
			"is_member":        r.UserID != nil,
			"last_active":      r.LastAct,
			"session_start":    r.CreatedAt,
		}
		if r.UserID != nil {
			av["user_id"] = *r.UserID
			av["user_name"] = strVal(r.UserName)
			av["user_email"] = strVal(r.UserEmail)
		}
		activeVisitors = append(activeVisitors, av)
	}

	// ---- chart_data ----
	chartData := make([]fiber.Map, 0, len(dailyRows))
	for _, r := range dailyRows {
		t, _ := time.Parse("2006-01-02", r.Date)
		chartData = append(chartData, fiber.Map{
			"name":      t.Format("02 Jan"),
			"full_date": r.Date,
			"visitors":  r.Visitors,
			"pageViews": r.PageViews,
		})
	}

	// ---- device_stats ----
	var mobile, tablet, desktop int64
	for _, r := range deviceRows {
		os := ""
		if r.OS != nil {
			os = *r.OS
		}
		switch {
		case containsAny(os, "Android", "iPhone", "iOS"):
			mobile += r.Count
		case containsAny(os, "iPad", "Tablet"):
			tablet += r.Count
		default:
			desktop += r.Count
		}
	}
	totalDevices := mobile + tablet + desktop
	deviceStats := []fiber.Map{
		{"name": "Desktop", "value": pct(desktop, totalDevices), "count": desktop, "color": "#63E6E2"},
		{"name": "Mobile", "value": pct(mobile, totalDevices), "count": mobile, "color": "#0EA5E9"},
		{"name": "Tablet", "value": pct(tablet, totalDevices), "count": tablet, "color": "#6366F1"},
	}

	// ---- traffic_sources ----
	srcMap := map[string]int64{}
	for _, r := range refRows {
		src := "Direct"
		if r.Referer != nil && *r.Referer != "" {
			src = extractDomain(*r.Referer)
		}
		srcMap[src] += r.Count
	}
	
	totalCurrent := s.repo.GetTotalVisitsSince(dbCode, since)
	changePerSource := 0.0
	if prevTotalVisits > 0 {
		changePerSource = float64(totalCurrent-prevTotalVisits) / float64(prevTotalVisits) * 100
	}
	trafficSources := make([]fiber.Map, 0, len(srcMap))
	for src, visits := range srcMap {
		trafficSources = append(trafficSources, fiber.Map{
			"source": src,
			"visits": visits,
			"change": changePerSource,
		})
	}

	if countryStats == nil {
		countryStats = []repositories.CountryRow{}
	}

	return fiber.Map{
		"visitor_stats": fiber.Map{
			"current":              currentActive,
			"current_members":      currentMembers,
			"current_guests":       currentGuests,
			"total_today":          totalToday,
			"total_combined_today": totalToday,
			"change":               changeVal,
			"history":              chartData,
			"active_visitors":      activeVisitors,
		},
		"user_stats": fiber.Map{
			"total":     totalUsers,
			"active":    activeUsers,
			"new_today": newToday,
		},
		"country_stats":   countryStats,
		"chart_data":      chartData,
		"device_stats":    deviceStats,
		"traffic_sources": trafficSources,
	}
}

func (s *analyticsService) PruneAnalytics(dbCode database.CountryID, days int) int64 {
	cutoff := time.Now().AddDate(0, 0, -days)
	return s.repo.PruneVisitorTracking(dbCode, cutoff)
}

func (s *analyticsService) GetDashboardSummary(dbCode database.CountryID) fiber.Map {
	fiveMinAgo := time.Now().Add(-5 * time.Minute)

	var wg sync.WaitGroup
	var articleCount, newsCount, userCount, onlineCount int64
	var artTrend, newsTrend, userTrend repositories.TrendRow

	wg.Add(2)
	go func() {
		defer wg.Done()
		articleCount, newsCount, userCount, onlineCount = s.repo.GetTotals(dbCode, fiveMinAgo)
	}()

	go func() {
		defer wg.Done()
		now := time.Now()
		thisMonthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		lastMonthStart := thisMonthStart.AddDate(0, -1, 0)
		artTrend, newsTrend, userTrend = s.repo.GetTrends(dbCode, thisMonthStart, lastMonthStart)
	}()

	wg.Wait()

	rawActivities, _ := s.repo.GetRecentActivities()

	type activityOut struct {
		ID        uint      `json:"id"`
		Type      string    `json:"type"`
		Title     string    `json:"title"`
		User      fiber.Map `json:"user"`
		CreatedAt time.Time `json:"created_at"`
	}
	activities := make([]activityOut, 0, len(rawActivities))
	for _, a := range rawActivities {
		atype := "article"
		if a.SubjectType != nil {
			switch *a.SubjectType {
			case "Post":
				atype = "news"
			case "Comment":
				atype = "comment"
			}
		}
		activities = append(activities, activityOut{
			ID:        a.ID,
			Type:      atype,
			Title:     a.Description,
			User:      fiber.Map{"name": a.CauserName},
			CreatedAt: a.CreatedAt,
		})
	}

	return fiber.Map{
		"totals": fiber.Map{
			"articles":     articleCount,
			"news":         newsCount,
			"users":        userCount,
			"online_users": onlineCount,
		},
		"trends": fiber.Map{
			"articles": trendData(artTrend.LastMonth, artTrend.ThisMonth),
			"news":     trendData(newsTrend.LastMonth, newsTrend.ThisMonth),
			"users":    trendData(userTrend.LastMonth, userTrend.ThisMonth),
		},
		"analytics": fiber.Map{
			"dates":    []string{},
			"articles": []int{},
			"news":     []int{},
			"comments": []int{},
			"views":    []int{},
			"authors":  []int{},
		},
		"onlineUsers":      []interface{}{},
		"recentActivities": activities,
	}
}

func (s *analyticsService) GetContentAnalytics(dbCode database.CountryID) fiber.Map {
	var wg sync.WaitGroup
	var topArticles []repositories.ArticleView
	var topPosts []repositories.PostView
	var publishedArticles, draftArticles int64

	wg.Add(3)
	go func() {
		defer wg.Done()
		topArticles, _ = s.repo.GetTopArticles(dbCode)
	}()
	go func() {
		defer wg.Done()
		topPosts, _ = s.repo.GetTopPosts(dbCode)
	}()
	go func() {
		defer wg.Done()
		publishedArticles, draftArticles = s.repo.GetArticleCountsByStatus(dbCode)
	}()

	wg.Wait()

	return fiber.Map{
		"top_articles":       topArticles,
		"top_posts":          topPosts,
		"published_articles": publishedArticles,
		"draft_articles":     draftArticles,
	}
}

// ---- helper funcs ----

func strVal(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func pct(part, total int64) float64 {
	if total == 0 {
		return 0
	}
	return float64(part) / float64(total) * 100
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if len(s) >= len(sub) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}

func extractDomain(rawURL string) string {
	s := rawURL
	for _, prefix := range []string{"https://", "http://"} {
		if len(s) > len(prefix) && s[:len(prefix)] == prefix {
			s = s[len(prefix):]
			break
		}
	}
	for i, ch := range s {
		if ch == '/' {
			return s[:i]
		}
	}
	return s
}

func trendData(prev, curr int64) fiber.Map {
	pct := 0.0
	dir := "up"
	if prev > 0 {
		pct = float64(curr-prev) / float64(prev) * 100
	}
	if pct < 0 {
		dir = "down"
	}
	return fiber.Map{"percentage": pct, "trend": dir}
}
