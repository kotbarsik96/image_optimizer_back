package main

import (
	"fmt"
	"sync"
)

type Progress[T any] struct {
	ActionID   uint
	UploaderID uint
	Stream     *ProgressStream
	Meta       T
	mu         sync.Mutex
	completed  uint
	total      uint
}

func (p *Progress[T]) Increment() {
	p.mu.Lock()
	p.completed += 1
	percent := p.GetPercent()
	p.mu.Unlock()

	select {
	case p.Stream.Value <- percent:
	default:
	}
}

func (p *Progress[T]) Finish() {
	p.mu.Lock()
	p.completed = p.total
	p.mu.Unlock()

	select {
	case p.Stream.Value <- 100:
	default:
	}
}

func (p *Progress[T]) GetPercent() float64 {
	total := float64(p.total)
	if total == 0 {
		return 0
	}
	return float64(p.completed) / total * 100
}

type TProgressesStorage[T any] struct {
	Name       string
	mu         sync.RWMutex
	progresses []*Progress[T]
}

func (ps *TProgressesStorage[T]) NewProgress(actionID uint, uploaderID, total uint) *Progress[T] {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	p := &Progress[T]{
		ActionID:   actionID,
		UploaderID: uploaderID,
		total:      total,
		Stream:     NewProgressStream(),
	}
	go p.Stream.listen()
	ps.progresses = append(ps.progresses, p)
	return p
}

func (ps *TProgressesStorage[T]) GetProgress(actionID uint) (*Progress[T], error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	for _, p := range ps.progresses {
		if p.ActionID == actionID {
			return p, nil
		}
	}
	return nil, fmt.Errorf("Progress %v.%v not found", ps.Name, actionID)
}

func (ps *TProgressesStorage[T]) GetUploaderProgresses(uploaderID uint) []*Progress[T] {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	list := []*Progress[T]{}
	for _, p := range ps.progresses {
		if p.UploaderID == uploaderID {
			list = append(list, p)
		}
	}
	return list
}

func (ps *TProgressesStorage[T]) FinishProgress(p *Progress[T]) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	p.Finish()
	ps.progresses = FilterSlice(ps.progresses, func(index int, item *Progress[T], slice []*Progress[T]) bool {
		if item.ActionID == p.ActionID {
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

type TOptimizationProgressStorageMeta struct{}

var OptimizationsProgressStorage TProgressesStorage[TOptimizationProgressStorageMeta] = TProgressesStorage[TOptimizationProgressStorageMeta]{
	Name:       "optimizations",
	progresses: []*Progress[TOptimizationProgressStorageMeta]{},
}

type TUploadProgressStorageMeta struct {
}

var UploadsProgressStorage TProgressesStorage[TUploadProgressStorageMeta] = TProgressesStorage[TUploadProgressStorageMeta]{
	Name:       "uploads",
	progresses: []*Progress[TUploadProgressStorageMeta]{},
}

type TProgressSyncActionsList map[string]TProgressSyncValuesList

type TProgressSyncValuesList map[uint]float64

func ProgressStorageToList[T any](list TProgressSyncActionsList, uploaderID uint, ps *TProgressesStorage[T]) {
	list[ps.Name] = TProgressSyncValuesList{}

	for _, progress := range ps.GetUploaderProgresses(uploaderID) {
		list[ps.Name][progress.ActionID] = progress.GetPercent()
	}
}
