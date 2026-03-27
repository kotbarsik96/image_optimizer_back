package main

import (
	"sync"
	"sync/atomic"
)

type EProgressAction string

const (
	EPrActOptimizations EProgressAction = "optimizations"
	EPrActUploads       EProgressAction = "uploads"
)

type UploadersProgressesStorage struct {
	mu      sync.RWMutex
	storage map[uint][]ActionProgress
}

type ActionProgress struct {
	ActionName EProgressAction
	ActionID   uint
	Progress   *Progress
}

func (ups *UploadersProgressesStorage) NewAction(u Uploader, actionName EProgressAction, actionID uint, total int64) ActionProgress {
	ups.mu.Lock()
	defer ups.mu.Unlock()

	actionProgress := ActionProgress{
		ActionName: actionName,
		ActionID:   actionID,
		Progress: &Progress{
			updates: make(chan float64, 1),
		},
	}

	_, ok := ups.storage[u.ID]
	if !ok {
		ups.storage[u.ID] = []ActionProgress{}
	}
	ups.storage[u.ID] = append(ups.storage[u.ID], actionProgress)

	return actionProgress
}

func (ups *UploadersProgressesStorage) Actions(u Uploader, actionName EProgressAction) []ActionProgress {
	ups.mu.RLock()
	defer ups.mu.RUnlock()

	list, ok := ups.storage[u.ID]
	if !ok {
		return []ActionProgress{}
	}

	return FilterSlice(list, func(index int, item ActionProgress, slice []ActionProgress) bool {
		return item.ActionName == actionName
	})
}

// не использует mu.Lock
func (ups *UploadersProgressesStorage) findAction(u Uploader, actionName EProgressAction, actionID uint) *ActionProgress {
	for _, ap := range ups.Actions(u, actionName) {
		if ap.ActionName == actionName && ap.ActionID == actionID {
			return &ap
		}
	}

	return nil
}

func (ups *UploadersProgressesStorage) FinishAction(u Uploader, action ActionProgress) {
	ups.mu.Lock()
	defer ups.mu.Unlock()

	list, ok := ups.storage[u.ID]
	if !ok {
		return
	}

	ups.storage[u.ID] = FilterSlice(list, func(index int, item ActionProgress, slice []ActionProgress) bool {
		if item.ActionID == action.ActionID && item.ActionName == action.ActionName {
			item.Progress.Finish()
			return false
		}
		return true
	})
}

var UploadersProgresses = UploadersProgressesStorage{
	storage: make(map[uint][]ActionProgress),
}

type Progress struct {
	completed atomic.Int64
	total     atomic.Int64
	updates   chan float64
}

func (p *Progress) SetTotal(val int64) {
	p.total.Store(val)
}

func (p *Progress) Increment() (done bool) {
	p.completed.Add(1)
	percent := p.GetPercent()

	select {
	case p.updates <- percent:
	default:
	}

	return percent >= 100
}

func (p *Progress) GetPercent() float64 {
	completed := p.completed.Load()
	if completed == 0 {
		return 0
	}
	total := p.total.Load()
	return float64(completed) / float64(total) * 100
}

func (p *Progress) GetUpdates() <-chan float64 {
	return p.updates
}

func (p *Progress) Finish() {
	close(p.updates)
}
