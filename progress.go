package main

import (
	"fmt"
	"maps"
	"sync"
)

type TProgressSSE struct {
	Value   float64
	Details TProgressDetails
}

type ProgressStatus int

const (
	ProgressCreated ProgressStatus = iota + 1
	ProgressPending
	ProgressDone
)

type IProgressStatusEntity interface {
	GetID() uint
	SetProgressStatus(ps ProgressStatus)
}

type TProgressDetails map[string]TProgressDetailItem

type TProgressDetailItem struct {
	Error error          `json:"error,omitzero"`
	Done  bool           `json:"done"`
	Meta  map[string]any `json:"meta,omitzero"`
}

type Progress struct {
	UploaderID uint
	Entity     IProgressStatusEntity
	Stream     *ProgressStream
	Details    TProgressDetails
	mu         sync.Mutex
	completed  uint
	total      uint
}

func (p *Progress) GetEntityID() uint {
	return p.Entity.GetID()
}

func (p *Progress) Increment() {
	p.mu.Lock()

	p.completed += 1

	p.mu.Unlock()

	p.StreamProgressPercent()
}

func (p *Progress) IncrementWithDetails(detailKey string, detailValue TProgressDetailItem) {
	p.mu.Lock()

	p.completed += 1
	if detailValue.Error == nil {
		detailValue.Done = true
	}
	p.Details[detailKey] = detailValue

	p.mu.Unlock()

	p.StreamProgressPercent()
}

func (p *Progress) StreamProgressPercent() {
	percent := p.GetPercent()

	select {
	case p.Stream.Value <- percent:
	default:
	}
}

func (p *Progress) Finish() {
	p.mu.Lock()
	p.completed = p.total
	p.mu.Unlock()

	select {
	case p.Stream.Value <- 100:
	default:
	}
}

func (p *Progress) GetPercent() float64 {
	total := float64(p.total)
	if total == 0 {
		return 0
	}
	return float64(p.completed) / total * 100
}

type TProgressesStorage struct {
	Name       string
	mu         sync.RWMutex
	progresses []*Progress
}

// если у данного entity ещё нет прогресса, создаст новый, записав туда total и details
//
// если у данного entity прогресс уже зарегистрирован - вернёт его, прибавив total и прибавив details.
//
// все дальнейшие действия с прогрессом должны идти через методы TProgressesStorage (Increment)
//
// прогресс будет завершён автоматически при достижении total == completed (при условии увеличения через методы [TProgressesStorage.Increment], [TProgressesStorage.IncrementWithDetails])
func (ps *TProgressesStorage) NewProgress(entity IProgressStatusEntity, uploaderID, total uint, details TProgressDetails) *Progress {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	p, err := ps.getProgressUnlocked(entity.GetID())
	entity.SetProgressStatus(ProgressPending)
	if err == nil {
		p = &Progress{
			UploaderID: uploaderID,
			Entity:     entity,
			total:      total,
			Stream:     NewProgressStream(),
			Details:    details,
		}
	} else {
		p.total += total
		maps.Copy(p.Details, details)
	}

	go p.Stream.listen()
	ps.progresses = append(ps.progresses, p)
	return p
}

// получить прогресс по entityID
func (ps *TProgressesStorage) GetProgress(entityID uint) (*Progress, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return ps.getProgressUnlocked(entityID)
}

// получить прогресс по entityID (без mu.RLock)
func (ps *TProgressesStorage) getProgressUnlocked(entityID uint) (*Progress, error) {
	for _, p := range ps.progresses {
		if p.GetEntityID() == entityID {
			return p, nil
		}
	}
	return nil, fmt.Errorf("Progress %v.%v not found", ps.Name, entityID)
}

// получить активные прогрессы из данного хранилища, привязанные к uploader'у по его ID
func (ps *TProgressesStorage) GetUploaderProgresses(uploaderID uint) []*Progress {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	list := []*Progress{}
	for _, p := range ps.progresses {
		if p.UploaderID == uploaderID {
			list = append(list, p)
		}
	}
	return list
}

func (ps *TProgressesStorage) Increment(p *Progress) {
	p.Increment()
	ps.afterIncrement(p)
}

func (ps *TProgressesStorage) IncrementWithDetails(p *Progress, detailKey string, detailValue TProgressDetailItem) {
	p.IncrementWithDetails(detailKey, detailValue)
	ps.afterIncrement(p)
}

func (ps *TProgressesStorage) afterIncrement(p *Progress) {
	if p.completed == p.total {
		ps.Finish(p)
	}
}

// завершает прогресс и удаляет его из хранилища
func (ps *TProgressesStorage) Finish(p *Progress) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	p.Finish()
	p.Entity.SetProgressStatus(ProgressDone)
	ps.progresses = FilterSlice(ps.progresses, func(index int, item *Progress, slice []*Progress) bool {
		if item.GetEntityID() == p.GetEntityID() {
			return false
		}
		return true
	})
}

type ProgressClientChan chan float64

type ProgressStream struct {
	Value ProgressClientChan

	NewClients chan ProgressClientChan

	ClosedClients chan ProgressClientChan

	TotalClients map[ProgressClientChan]bool
}

func (ps *ProgressStream) listen() {
	defer func() {
		for clientChan := range ps.TotalClients {
			close(clientChan)
			delete(ps.TotalClients, clientChan)
		}
	}()

	for {
		select {
		case client := <-ps.NewClients:
			ps.TotalClients[client] = true

		case client := <-ps.ClosedClients:
			delete(ps.TotalClients, client)

		case value, ok := <-ps.Value:
			if !ok {
				return
			}

			for clientChan := range ps.TotalClients {
				select {
				case clientChan <- value:
					// доставлено
				default:
					// не доставлено
				}
			}

			if value >= 100 {
				return
			}
		}
	}
}

func NewProgressStream() *ProgressStream {
	return &ProgressStream{
		Value:         make(ProgressClientChan),
		NewClients:    make(chan ProgressClientChan),
		ClosedClients: make(chan ProgressClientChan),
		TotalClients:  make(map[ProgressClientChan]bool),
	}
}

// регистрация TProgressStorage'й

var OptimizationsProgressStorage TProgressesStorage = TProgressesStorage{
	Name:       "optimizations",
	progresses: []*Progress{},
}

var UploadsProgressStorage TProgressesStorage = TProgressesStorage{
	Name:       "uploads",
	progresses: []*Progress{},
}
