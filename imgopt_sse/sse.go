package imgopt_sse

import (
	"log"

	"github.com/gin-gonic/gin"
)

type Event struct {
	Message chan string

	NewClients chan chan string

	ClosedClients chan chan string

	TotalClients map[chan string]bool
}

type ClientChan chan string

func NewServer() (event *Event) {
	event = &Event{
		Message:       make(chan string),
		NewClients:    make(chan chan string),
		ClosedClients: make(chan chan string),
		TotalClients:  make(map[chan string]bool),
	}

	go event.listen()

	return event
}

// Входящие запросы от клиентов: добавление, удаление клиентов и отправка сообщений
func (stream *Event) listen() {
	for {
		select {
		case client := <-stream.NewClients:
			stream.TotalClients[client] = true
			log.Printf("Client added. %d registered clients", len(stream.TotalClients))

		case client := <-stream.ClosedClients:
			delete(stream.TotalClients, client)
			close(client)
			log.Printf("Removed client. %d registered clients", len(stream.TotalClients))

		case eventMsg := <-stream.Message:
			for clientMessageChan := range stream.TotalClients {
				select {
				case clientMessageChan <- eventMsg:
					// Сообщение доставлено
				default:
					// Не удалось доставить сообщение
				}
			}
		}
	}
}

func (stream *Event) ServeHTTP() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Новый канал для клиента
		clientChan := make(ClientChan)

		// Оповестить о присоединении нового клиента
		stream.NewClients <- clientChan

		go func() {
			// Ожидает, когда клиент отсоединится
			<-c.Writer.CloseNotify()

			// Опустошить канал клиента
			for range clientChan {
			}

			// Оповестить об отсоединении клиента
			stream.ClosedClients <- clientChan
		}()

		c.Set("clientChan", clientChan)

		c.Next()
	}
}

func HeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")
		c.Writer.Header().Set("Transfer-Encoding", "chunked")

		c.Next()
	}
}
