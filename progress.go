package main

import (
	"maps"
	"sync"
)

// список элементов прогресса. Например, для "uploads" - изображения, то есть ключами являются названия изображений
type TProgressDetails map[string]TProgressDetailItem

type TProgressDetailItem struct {
	// ошибка записывается в это поле
	Error error `json:"error,omitzero"`
	// завершены ли действия по обработке элемента. Если error != nil - всегда будет false
	Done bool `json:"done"`
	// дополнительные данные
	Meta map[string]any `json:"meta,omitzero"`
}

// статус прогресса для хранения у сущности в базе
type ProgressStatus int

const (
	ProgressCreated ProgressStatus = iota + 1
	ProgressPending
	ProgressDone
)

// сущность прогресса: таблица в базе данных
type IProgressEntity interface {
	GetID() uint
	// изменяет статус прогресса сущности, обновляя запись в бд
	SetProgressStatus(ps ProgressStatus)
}

type TProgress struct {
	Details    TProgressDetails
	Entity     IProgressEntity
	UploaderID uint
	completed  uint
	total      uint
	mu         sync.Mutex
}

func (p *TProgress) GetID() uint {
	return p.Entity.GetID()
}

func (p *TProgress) GetPercent() float64 {
	total := float64(p.total)
	if total == 0 {
		return 0
	}
	return float64(p.completed) / total * 100
}

func (p *TProgress) Increment() {
	p.mu.Lock()

	if !p.Done() {
		p.completed += 1
	}

	p.mu.Unlock()

	ProgressSubscriptions.Broadcast(p)
}

func (p *TProgress) IncrementWithDetails(detailKey string, detailErr error, detailMeta map[string]any) {
	p.mu.Lock()

	if !p.Done() {
		detailValue := TProgressDetailItem{
			Error: detailErr,
			Meta:  detailMeta,
		}
		if detailErr == nil {
			detailValue.Done = true
		}

		p.Details[detailKey] = detailValue
		p.completed += 1
	}

	p.mu.Unlock()

	ProgressSubscriptions.Broadcast(p)
}

func (p *TProgress) GetData() TProgressSSE {
	return TProgressSSE{
		EntityID: p.GetID(),
		Value:    p.GetPercent(),
		Details:  p.Details,
	}
}

func (p *TProgress) Done() bool {
	return p.completed >= p.total
}

// формат данных для отправки по SSE
type TProgressSSE struct {
	EntityID uint             `json:"entity_id"`
	Value    float64          `json:"value"`
	Details  TProgressDetails `json:"details"`
}

type TProgressClientChan chan TProgressSSE

type TProgressSubscriptions struct {
	Clients map[*TProgress]map[TProgressClientChan]bool
	mu      sync.Mutex
}

func (ps *TProgressSubscriptions) Subscribe(progress *TProgress, inbox TProgressClientChan) {
	ps.mu.Lock()
	ps.subscribeUnlocked(progress, inbox)
	ps.mu.Unlock()
}

func (ps *TProgressSubscriptions) Unsubscribe(progress *TProgress, inbox TProgressClientChan) {
	ps.mu.Lock()
	ps.unsubscribeUnlocked(progress, inbox)
	ps.mu.Unlock()
}

func (ps *TProgressSubscriptions) subscribeUnlocked(progress *TProgress, inbox TProgressClientChan) {
	if _, ok := ps.Clients[progress]; !ok {
		ps.Clients[progress] = make(map[TProgressClientChan]bool, 0)
	}
	ps.Clients[progress][inbox] = true
	inbox <- progress.GetData()
}

func (ps *TProgressSubscriptions) unsubscribeUnlocked(progress *TProgress, inbox TProgressClientChan) {
	if _, ok := ps.Clients[progress]; ok {
		delete(ps.Clients[progress], inbox)
		close(inbox)
	}
	if len(ps.Clients[progress]) < 1 {
		delete(ps.Clients, progress)
	}
}

func (ps *TProgressSubscriptions) Broadcast(progress *TProgress) {
	ps.mu.Lock()

	clients, ok := ps.Clients[progress]
	if ok {
		for inbox := range clients {
			inbox <- progress.GetData()

			if progress.Done() {
				ps.unsubscribeUnlocked(progress, inbox)
			}
		}
	}

	if progress.Done() {
		progress.Entity.SetProgressStatus(ProgressDone)
	}

	ps.mu.Unlock()
}

// когда стартует новый прогресс - подписать подходящие inbox'ы на него
func (ps *TProgressSubscriptions) SubscribeToNewProgress(uploaderID uint, progress *TProgress) {
	ps.mu.Lock()

	for p, inboxes := range ps.Clients {
		if p.UploaderID == uploaderID {
			if p.GetID() == progress.GetID() {
				continue
			}

			for inbox := range inboxes {
				ps.subscribeUnlocked(p, inbox)
			}
		}
	}

	ps.mu.Unlock()
}

var ProgressSubscriptions TProgressSubscriptions = TProgressSubscriptions{
	Clients: make(map[*TProgress]map[TProgressClientChan]bool),
}

type TProgressesStorage struct {
	List []*TProgress
	mu   sync.Mutex
}

func (ps *TProgressesStorage) GetListByUploader(uploaderID uint) []*TProgress {
	return FilterSlice(ps.List, func(index int, item *TProgress, slice []*TProgress) bool {
		return item.UploaderID == uploaderID
	})
}

func (ps *TProgressesStorage) NewProgress(uploaderID uint, entity IProgressEntity, total uint, details TProgressDetails) *TProgress {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	var progress *TProgress
	for _, p := range ps.List {
		if p.UploaderID == uploaderID && p.GetID() == entity.GetID() {
			progress = p
		}
	}

	if progress == nil {
		progress = &TProgress{
			Details:    details,
			Entity:     entity,
			UploaderID: uploaderID,
			total:      total,
		}

		ProgressSubscriptions.SubscribeToNewProgress(uploaderID, progress)
	} else {
		progress.total += total
		maps.Copy(progress.Details, details)
	}

	return progress
}

var OptimizationsProgressStorage TProgressesStorage = TProgressesStorage{
	List: []*TProgress{},
}

var UploadsProgressStorage TProgressesStorage = TProgressesStorage{
	List: []*TProgress{},
}
