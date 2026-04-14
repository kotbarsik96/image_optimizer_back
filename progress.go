package main

import (
	"maps"
	"sync"
)

const SSE_STREAM_CLOSED = "stream_closed"

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
	GetProgressStatus() ProgressStatus
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

		p.AfterIncrement()
	}

	p.mu.Unlock()
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

		p.AfterIncrement()
	}

	p.mu.Unlock()
}

func (p *TProgress) AfterIncrement() {
	if p.Done() {
		p.Entity.SetProgressStatus(ProgressDone)
	}
	ProgressSubscriptions.Broadcast(p)
}

func (p *TProgress) GetData() TProgressSSE {
	return TProgressSSE{
		EntityID: p.GetID(),
		Value:    p.GetPercent(),
		Details:  p.Details,
		Done:     p.Done(),
	}
}

func (p *TProgress) Done() bool {
	return p.completed >= p.total
}

// формат данных для отправки по SSE
type TProgressSSE struct {
	EntityID uint             `json:"entity_id"`
	Value    float64          `json:"progress_value"`
	Details  TProgressDetails `json:"details"`
	Done     bool             `json:"done"`
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

		if len(ps.Clients[progress]) < 1 {
			delete(ps.Clients, progress)
		}
	}
}

// отправляет обновлённое значение прогресса подписавшимся слушателям
//
// при достижении progress.Done() отписывает слушателя (inbox) и выставляет progress.Entity статус ProgressDone
func (ps *TProgressSubscriptions) Broadcast(progress *TProgress) {
	ps.mu.Lock()

	clients, ok := ps.Clients[progress]
	if ok {
		for inbox := range clients {
			inbox <- progress.GetData()

			if progress.Done() {
				ps.unsubscribeUnlocked(progress, inbox)

				if !ps.InboxHasListeners(inbox) {
					close(inbox)
				}
			}
		}
	}

	ps.mu.Unlock()
}

// когда стартует новый прогресс - подписать подходящие inbox'ы на него - синхронизировать уже подключившихся ранее слушателей
func (ps *TProgressSubscriptions) SubscribeToNewProgress(uploaderID uint, newProgress *TProgress) {
	ps.mu.Lock()

	for oldProgress, inboxes := range ps.Clients {
		if oldProgress.UploaderID == uploaderID {
			if oldProgress.GetID() == newProgress.GetID() {
				continue
			}

			for inbox := range inboxes {
				ps.subscribeUnlocked(newProgress, inbox)
			}
		}
	}

	ps.mu.Unlock()
}

// возвращает true, если inbox на что-либо подписан
func (ps *TProgressSubscriptions) InboxHasListeners(inbox TProgressClientChan) bool {
	var has bool
	for _, inboxes := range ps.Clients {
		if _, ok := inboxes[inbox]; ok {
			has = true
		}
	}
	return has
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

// регистрация нового прогресса:
//
// если не зарегистрирован - создаст новый, добавит его в List и начнёт отслеживать его: по достижению progress.Done() == true прогресс будет удалён из List
//
// если уже зарегистрирован - добавит к существующему total и details
func (ps *TProgressesStorage) NewProgress(uploaderID uint, entity IProgressEntity, total uint, details TProgressDetails) *TProgress {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	entity.SetProgressStatus(ProgressPending)

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

		ps.List = append(ps.List, progress)

		// подписать подходящие inbox'ы на этот прогресс: синхронизировать с уже подключёнными клиентами
		ProgressSubscriptions.SubscribeToNewProgress(uploaderID, progress)

		// отслеживать прогресс, чтобы убрать его из List по достижению progress.Done() == true
		inbox := make(TProgressClientChan, 1)
		ProgressSubscriptions.Subscribe(progress, inbox)
		go func() {
			for {
				// пока прогресс не завершён - цикл продолжается
				if _, ok := <-inbox; ok {
					if !progress.Done() {
						continue
					}
				}

				// прогресс завершён:

				// отписка inbox'а произойдёт автоматически - остаётся только удалить progress из List
				ps.RemoveProgress(progress)
				return
			}
		}()
	} else {
		progress.total += total
		maps.Copy(progress.Details, details)
	}

	return progress
}

// удалить прогресс из List. Должен вызываться только после того, как прогресс завершился (progress.Done() == true) - иначе завершит прогресс "рвано"
func (ps *TProgressesStorage) RemoveProgress(progress *TProgress) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	// если прогресс не завершён - завершить его, что автоматически проивзедёт очистку слушателей
	for {
		if progress.Done() {
			break
		}
		progress.Increment()
	}

	ps.List = FilterSlice(ps.List, func(index int, item *TProgress, slice []*TProgress) bool {
		return item.GetID() != progress.GetID()
	})
}

var OptimizationsProgressStorage TProgressesStorage = TProgressesStorage{
	List: []*TProgress{},
}

var UploadsProgressStorage TProgressesStorage = TProgressesStorage{
	List: []*TProgress{},
}
