package main

import (
	"fmt"
	"sync"
)

type EProgressActionName string

const (
	EProgressStorageOptimizations EProgressActionName = "optimizations"
)

type Progress struct {
	ActionName EProgressActionName
	ActionID   uint
	Stream     *ProgressStream
	mu         sync.Mutex
	completed  uint
	total      uint
}

func (p *Progress) Increment() {
	p.mu.Lock()
	p.completed += 1
	percent := p.getPercent()
	p.mu.Unlock()

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

func (p *Progress) getPercent() float64 {
	total := float64(p.total)
	if total == 0 {
		return 0
	}
	return float64(p.completed) / total * 100
}

type TProgressesStorage struct {
	mu         sync.RWMutex
	progresses []*Progress
}

func (ps *TProgressesStorage) NewProgress(actionName EProgressActionName, actionID uint, total uint) *Progress {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	p := &Progress{
		ActionName: actionName,
		ActionID:   actionID,
		total:      total,
		Stream:     NewProgressStream(),
	}
	go p.Stream.listen()
	ps.progresses = append(ps.progresses, p)
	return p
}

func (ps *TProgressesStorage) GetProgress(actionName EProgressActionName, actionID uint) (*Progress, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	for _, p := range ps.progresses {
		if p.ActionName == actionName && p.ActionID == actionID {
			return p, nil
		}
	}
	return nil, fmt.Errorf("Progress %v.%v not found", actionName, actionID)
}

func (ps *TProgressesStorage) FinishProgress(p *Progress) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	p.Finish()
	ps.progresses = FilterSlice(ps.progresses, func(index int, item *Progress, slice []*Progress) bool {
		if item.ActionName == p.ActionName && item.ActionID == p.ActionID {
			return false
		}
		return true
	})
}

var ProgressesStorage TProgressesStorage = TProgressesStorage{
	progresses: []*Progress{},
}

type ProgressClientChan chan float64

type ProgressStream struct {
	Value ProgressClientChan

	NewClients chan ProgressClientChan

	ClosedClients chan ProgressClientChan

	TotalClients map[ProgressClientChan]bool
}

func (ps *ProgressStream) listen() {
loop:
	for {
		select {
		case client := <-ps.NewClients:
			ps.TotalClients[client] = true

		case client := <-ps.ClosedClients:
			delete(ps.TotalClients, client)

		case value := <-ps.Value:
			for clientChan := range ps.TotalClients {
				select {
				case clientChan <- value:
					// доставлено
				default:
					// не доставлено
				}
			}

			if value >= 100 {
				break loop
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
