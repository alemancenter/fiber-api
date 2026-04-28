package repositories

import (
	"time"

	"github.com/alemancenter/fiber-api/internal/database"
	"github.com/alemancenter/fiber-api/internal/models"
)

type AnalyticsRepository interface {
	GetVisitorStats(dbCode database.CountryID, activeWindow, todayStart, yesterdayStart time.Time) (currentActive, currentMembers, currentGuests, totalToday, totalYesterday int64)
	GetActiveVisitors(dbCode database.CountryID, activeWindow time.Time) ([]ActiveRow, error)
	GetUserStats(todayStart, activeWindow time.Time) (totalUsers, activeUsers, newToday int64)
	GetCountryStats(dbCode database.CountryID, since time.Time) ([]CountryRow, error)
	GetDailyChartData(dbCode database.CountryID, since time.Time) ([]DailyRow, error)
	GetDeviceStats(dbCode database.CountryID, since time.Time) ([]DeviceRow, error)
	GetTrafficSources(dbCode database.CountryID, since time.Time) ([]RefRow, error)
	GetPrevTotalVisits(dbCode database.CountryID, prevSince, since time.Time) int64
	GetTotalVisitsSince(dbCode database.CountryID, since time.Time) int64
	PruneVisitorTracking(dbCode database.CountryID, cutoff time.Time) int64

	GetTotals(dbCode database.CountryID, fiveMinAgo time.Time) (articleCount, newsCount, userCount, onlineCount int64)
	GetTrends(dbCode database.CountryID, thisMonthStart, lastMonthStart time.Time) (artTrend, newsTrend, userTrend TrendRow)
	GetRecentActivities() ([]ActivityRow, error)

	GetTopArticles(dbCode database.CountryID) ([]ArticleView, error)
	GetTopPosts(dbCode database.CountryID) ([]PostView, error)
	GetArticleCountsByStatus(dbCode database.CountryID) (published, draft int64)
}

type analyticsRepository struct{}

func NewAnalyticsRepository() AnalyticsRepository {
	return &analyticsRepository{}
}

// ─── Structs for raw scans ──────────────────────────────────────────────────

type ActiveRow struct {
	IPAddress string  `gorm:"column:ip_address"`
	Country   *string `gorm:"column:country"`
	City      *string `gorm:"column:city"`
	Browser   *string `gorm:"column:browser"`
	OS        *string `gorm:"column:os"`
	UserAgent string  `gorm:"column:user_agent"`
	URL       *string `gorm:"column:url"`
	UserID    *uint   `gorm:"column:user_id"`
	UserName  *string `gorm:"column:user_name"`
	UserEmail *string `gorm:"column:user_email"`
	LastAct   string  `gorm:"column:last_activity"`
	CreatedAt string  `gorm:"column:created_at"`
}

type CountryRow struct {
	Country string `gorm:"column:country" json:"country"`
	Count   int64  `gorm:"column:count"   json:"count"`
}

type DailyRow struct {
	Date      string `gorm:"column:date"`
	Visitors  int64  `gorm:"column:visitors"`
	PageViews int64  `gorm:"column:page_views"`
}

type DeviceRow struct {
	OS    *string `gorm:"column:os"`
	Count int64   `gorm:"column:count"`
}

type RefRow struct {
	Referer *string `gorm:"column:referer"`
	Count   int64   `gorm:"column:count"`
}

type TrendRow struct {
	ThisMonth int64 `gorm:"column:this_month"`
	LastMonth int64 `gorm:"column:last_month"`
}

type ActivityRow struct {
	ID          uint      `gorm:"column:id"`
	Description string    `gorm:"column:description"`
	SubjectType *string   `gorm:"column:subject_type"`
	CauserName  string    `gorm:"column:causer_name"`
	CreatedAt   time.Time `gorm:"column:created_at"`
}

type ArticleView struct {
	ID         uint   `json:"id"`
	Title      string `json:"title"`
	VisitCount int    `json:"visit_count"`
}

type PostView struct {
	ID    uint   `json:"id"`
	Title string `json:"title"`
	Views int    `json:"views"`
}

// ─── Implementations ─────────────────────────────────────────────────────────

func (r *analyticsRepository) GetVisitorStats(dbCode database.CountryID, activeWindow, todayStart, yesterdayStart time.Time) (currentActive, currentMembers, currentGuests, totalToday, totalYesterday int64) {
	db := database.DBForCountry(dbCode)
	db.Model(&models.VisitorTracking{}).Where("last_activity >= ?", activeWindow).Count(&currentActive)
	db.Model(&models.VisitorTracking{}).Where("last_activity >= ? AND user_id IS NOT NULL", activeWindow).Count(&currentMembers)
	db.Model(&models.VisitorTracking{}).Where("last_activity >= ? AND user_id IS NULL", activeWindow).Count(&currentGuests)
	db.Model(&models.VisitorTracking{}).Where("created_at >= ?", todayStart).Count(&totalToday)
	db.Model(&models.VisitorTracking{}).Where("created_at >= ? AND created_at < ?", yesterdayStart, todayStart).Count(&totalYesterday)
	return
}

func (r *analyticsRepository) GetActiveVisitors(dbCode database.CountryID, activeWindow time.Time) ([]ActiveRow, error) {
	db := database.DBForCountry(dbCode)
	var activeRows []ActiveRow
	err := db.Raw(`SELECT vt.ip_address, vt.country, vt.city, vt.browser, vt.os, vt.user_agent,
		               vt.url, vt.user_id, u.name AS user_name, u.email AS user_email,
		               vt.last_activity, vt.created_at
		        FROM visitors_tracking vt
		        LEFT JOIN users u ON u.id = vt.user_id
		        WHERE vt.last_activity >= ?
		        ORDER BY vt.last_activity DESC LIMIT 100`, activeWindow).Scan(&activeRows).Error
	return activeRows, err
}

func (r *analyticsRepository) GetUserStats(todayStart, activeWindow time.Time) (totalUsers, activeUsers, newToday int64) {
	mainDB := database.DB()
	mainDB.Model(&models.User{}).Count(&totalUsers)
	mainDB.Model(&models.User{}).Where("last_activity >= ?", activeWindow).Count(&activeUsers)
	mainDB.Model(&models.User{}).Where("created_at >= ?", todayStart).Count(&newToday)
	return
}

func (r *analyticsRepository) GetCountryStats(dbCode database.CountryID, since time.Time) ([]CountryRow, error) {
	db := database.DBForCountry(dbCode)
	var countryStats []CountryRow
	err := db.Raw(`SELECT COALESCE(country, 'Unknown') AS country, COUNT(*) AS count
		        FROM visitors_tracking
		        WHERE created_at >= ? AND country IS NOT NULL
		        GROUP BY country ORDER BY count DESC LIMIT 20`, since).Scan(&countryStats).Error
	return countryStats, err
}

func (r *analyticsRepository) GetDailyChartData(dbCode database.CountryID, since time.Time) ([]DailyRow, error) {
	db := database.DBForCountry(dbCode)
	var dailyRows []DailyRow
	err := db.Raw(`SELECT DATE(created_at) AS date,
		               COUNT(DISTINCT ip_address) AS visitors,
		               COUNT(*) AS page_views
		        FROM visitors_tracking
		        WHERE created_at >= ?
		        GROUP BY DATE(created_at) ORDER BY date ASC`, since).Scan(&dailyRows).Error
	return dailyRows, err
}

func (r *analyticsRepository) GetDeviceStats(dbCode database.CountryID, since time.Time) ([]DeviceRow, error) {
	db := database.DBForCountry(dbCode)
	var deviceRows []DeviceRow
	err := db.Raw(`SELECT os, COUNT(*) AS count FROM visitors_tracking
		        WHERE created_at >= ?
		        GROUP BY os`, since).Scan(&deviceRows).Error
	return deviceRows, err
}

func (r *analyticsRepository) GetTrafficSources(dbCode database.CountryID, since time.Time) ([]RefRow, error) {
	db := database.DBForCountry(dbCode)
	var refRows []RefRow
	err := db.Raw(`SELECT referer, COUNT(*) AS count FROM visitors_tracking
		        WHERE created_at >= ?
		        GROUP BY referer ORDER BY count DESC LIMIT 50`, since).Scan(&refRows).Error
	return refRows, err
}

func (r *analyticsRepository) GetPrevTotalVisits(dbCode database.CountryID, prevSince, since time.Time) int64 {
	db := database.DBForCountry(dbCode)
	var prevTotalVisits int64
	db.Model(&models.VisitorTracking{}).Where("created_at >= ? AND created_at < ?", prevSince, since).Count(&prevTotalVisits)
	return prevTotalVisits
}

func (r *analyticsRepository) GetTotalVisitsSince(dbCode database.CountryID, since time.Time) int64 {
	db := database.DBForCountry(dbCode)
	var totalCurrent int64
	db.Model(&models.VisitorTracking{}).Where("created_at >= ?", since).Count(&totalCurrent)
	return totalCurrent
}

func (r *analyticsRepository) PruneVisitorTracking(dbCode database.CountryID, cutoff time.Time) int64 {
	db := database.DBForCountry(dbCode)
	result := db.Where("created_at < ?", cutoff).Delete(&models.VisitorTracking{})
	return result.RowsAffected
}

func (r *analyticsRepository) GetTotals(dbCode database.CountryID, fiveMinAgo time.Time) (articleCount, newsCount, userCount, onlineCount int64) {
	db := database.DBForCountry(dbCode)
	mainDB := database.DB()
	db.Model(&models.Article{}).Where("status = ?", 1).Count(&articleCount)
	db.Model(&models.Post{}).Where("is_active = ?", true).Count(&newsCount)
	mainDB.Model(&models.User{}).Count(&userCount)
	mainDB.Model(&models.User{}).Where("last_activity >= ?", fiveMinAgo).Count(&onlineCount)
	return
}

func (r *analyticsRepository) GetTrends(dbCode database.CountryID, thisMonthStart, lastMonthStart time.Time) (artTrend, newsTrend, userTrend TrendRow) {
	db := database.DBForCountry(dbCode)
	mainDB := database.DB()

	db.Raw(`
			SELECT
			  SUM(CASE WHEN created_at >= ? THEN 1 ELSE 0 END) AS this_month,
			  SUM(CASE WHEN created_at >= ? AND created_at < ? THEN 1 ELSE 0 END) AS last_month
			FROM articles
			WHERE status = 1`,
		thisMonthStart, lastMonthStart, thisMonthStart,
	).Scan(&artTrend)

	db.Raw(`
			SELECT
			  SUM(CASE WHEN created_at >= ? THEN 1 ELSE 0 END) AS this_month,
			  SUM(CASE WHEN created_at >= ? AND created_at < ? THEN 1 ELSE 0 END) AS last_month
			FROM posts
			WHERE is_active = 1`,
		thisMonthStart, lastMonthStart, thisMonthStart,
	).Scan(&newsTrend)

	mainDB.Raw(`
			SELECT
			  SUM(CASE WHEN created_at >= ? THEN 1 ELSE 0 END) AS this_month,
			  SUM(CASE WHEN created_at >= ? AND created_at < ? THEN 1 ELSE 0 END) AS last_month
			FROM users`,
		thisMonthStart, lastMonthStart, thisMonthStart,
	).Scan(&userTrend)

	return
}

func (r *analyticsRepository) GetRecentActivities() ([]ActivityRow, error) {
	mainDB := database.DB()
	var rawActivities []ActivityRow
	err := mainDB.Raw(`
		SELECT al.id, al.description, al.subject_type, al.created_at,
		       COALESCE(u.name, 'مستخدم') AS causer_name
		FROM activity_log al
		LEFT JOIN users u ON u.id = al.causer_id
		ORDER BY al.created_at DESC
		LIMIT 10`,
	).Scan(&rawActivities).Error
	return rawActivities, err
}

func (r *analyticsRepository) GetTopArticles(dbCode database.CountryID) ([]ArticleView, error) {
	db := database.DBForCountry(dbCode)
	var topArticles []ArticleView
	err := db.Model(&models.Article{}).
		Select("id, title, visit_count").
		Where("status = ?", 1).
		Order("visit_count DESC").
		Limit(10).
		Scan(&topArticles).Error
	return topArticles, err
}

func (r *analyticsRepository) GetTopPosts(dbCode database.CountryID) ([]PostView, error) {
	db := database.DBForCountry(dbCode)
	var topPosts []PostView
	err := db.Model(&models.Post{}).
		Select("id, title, views").
		Where("is_active = ?", true).
		Order("views DESC").
		Limit(10).
		Scan(&topPosts).Error
	return topPosts, err
}

func (r *analyticsRepository) GetArticleCountsByStatus(dbCode database.CountryID) (published, draft int64) {
	db := database.DBForCountry(dbCode)
	db.Model(&models.Article{}).Where("status = ?", 1).Count(&published)
	db.Model(&models.Article{}).Where("status = ?", 0).Count(&draft)
	return
}
