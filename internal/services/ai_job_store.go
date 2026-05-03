package services

import (
	"sync"
	"time"
)

// JobStatus represents the lifecycle state of an async AI job.
type JobStatus string

const (
	JobPending JobStatus = "pending"
	JobDone    JobStatus = "done"
	JobFailed  JobStatus = "failed"

	// jobTTL is how long a completed/failed job is kept before being pruned.
	jobTTL = 15 * time.Minute
)

// AIJob holds the state and result of one AI generation request.
type AIJob struct {
	ID        string
	Status    JobStatus
	Content   string
	Error     string
	CreatedAt time.Time
}

// AIJobStore is a thread-safe in-memory store for async AI jobs.
type AIJobStore struct {
	mu   sync.RWMutex
	jobs map[string]*AIJob
}

var globalAIJobStore = &AIJobStore{jobs: make(map[string]*AIJob)}

// GetAIJobStore returns the singleton job store.
func GetAIJobStore() *AIJobStore { return globalAIJobStore }

// Create registers a new pending job and returns it.
func (s *AIJobStore) Create(id string) *AIJob {
	job := &AIJob{ID: id, Status: JobPending, CreatedAt: time.Now()}
	s.mu.Lock()
	s.jobs[id] = job
	s.mu.Unlock()
	return job
}

// Get looks up a job. Returns (nil, false) if not found or expired.
func (s *AIJobStore) Get(id string) (*AIJob, bool) {
	s.mu.RLock()
	job, ok := s.jobs[id]
	s.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if time.Since(job.CreatedAt) > jobTTL {
		s.mu.Lock()
		delete(s.jobs, id)
		s.mu.Unlock()
		return nil, false
	}
	return job, true
}

// Complete marks a job as done with its generated content.
func (s *AIJobStore) Complete(id, content string) {
	s.mu.Lock()
	if job, ok := s.jobs[id]; ok {
		job.Status = JobDone
		job.Content = content
	}
	s.mu.Unlock()
}

// Fail marks a job as failed with an error message.
func (s *AIJobStore) Fail(id, errMsg string) {
	s.mu.Lock()
	if job, ok := s.jobs[id]; ok {
		job.Status = JobFailed
		job.Error = errMsg
	}
	s.mu.Unlock()
}

// Prune removes all jobs older than jobTTL. Call periodically from a goroutine.
func (s *AIJobStore) Prune() {
	cutoff := time.Now().Add(-jobTTL)
	s.mu.Lock()
	for id, job := range s.jobs {
		if job.CreatedAt.Before(cutoff) {
			delete(s.jobs, id)
		}
	}
	s.mu.Unlock()
}
