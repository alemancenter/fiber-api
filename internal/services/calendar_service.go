package services

import (
	"github.com/alemancenter/fiber-api/internal/database"
	"github.com/alemancenter/fiber-api/internal/models"
	"github.com/alemancenter/fiber-api/internal/repositories"
)

type CalendarService interface {
	ListEvents(countryID database.CountryID, start, end string) ([]models.Event, error)
	GetEvent(countryID database.CountryID, id uint64) (*models.Event, error)
	CreateEvent(countryID database.CountryID, event *models.Event) error
	UpdateEvent(countryID database.CountryID, id uint64, updates map[string]interface{}) (*models.Event, error)
	DeleteEvent(countryID database.CountryID, id uint64) error
	ListPublicEvents(countryID database.CountryID, limit int) ([]models.Event, error)
}

type calendarService struct {
	repo repositories.CalendarRepository
}

func NewCalendarService(repo repositories.CalendarRepository) CalendarService {
	return &calendarService{repo: repo}
}

func (s *calendarService) ListEvents(countryID database.CountryID, start, end string) ([]models.Event, error) {
	return s.repo.ListEvents(countryID, start, end)
}

func (s *calendarService) GetEvent(countryID database.CountryID, id uint64) (*models.Event, error) {
	return s.repo.FindEventByID(countryID, id)
}

func (s *calendarService) CreateEvent(countryID database.CountryID, event *models.Event) error {
	return s.repo.CreateEvent(countryID, event)
}

func (s *calendarService) UpdateEvent(countryID database.CountryID, id uint64, updates map[string]interface{}) (*models.Event, error) {
	event, err := s.repo.FindEventByID(countryID, id)
	if err != nil {
		return nil, err
	}

	if err := s.repo.UpdateEvent(countryID, event, updates); err != nil {
		return nil, err
	}

	return event, nil
}

func (s *calendarService) DeleteEvent(countryID database.CountryID, id uint64) error {
	if _, err := s.repo.FindEventByID(countryID, id); err != nil {
		return err
	}

	return s.repo.DeleteEvent(countryID, id)
}

func (s *calendarService) ListPublicEvents(countryID database.CountryID, limit int) ([]models.Event, error) {
	return s.repo.ListPublicEvents(countryID, limit)
}