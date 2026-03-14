package main

import (
	"fmt"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type TUploaderEntity struct {
	Id   int    `json:"id"`
	Uuid string `json:"uuid"`
}

func NewUploaderEntity(c *gin.Context) (TUploaderEntity, error) {
	uploaderUuid := uuid.New().String()
	uploader := TUploaderEntity{
		Uuid: uploaderUuid,
	}

	err := uploader.Save()
	if err != nil {
		fmt.Printf("Error while trying to save uploader session: %v\n", err)
		return uploader, err
	}

	return uploader, nil
}

func (uploader *TUploaderEntity) Save() error {
	id, err := dbwrapper.SaveEntity("uploaders", uploader)
	uploader.Id = id
	return err
}

// вернёт текущего пользователя, создав его при отсутствии
func GetCurrentUploader(c *gin.Context) TUploaderEntity {
	db := dbwrapper.DB
	session := sessions.Default(c)
	uploaderUuid := session.Get("uploader")

	uploader := TUploaderEntity{}

	db.QueryRow("SELECT * FROM uploaders WHERE uuid = ?", uploaderUuid).
		Scan(&uploader.Id, &uploader.Uuid)

	// uploader ещё не создан: создать
	if uploader.Id == 0 {
		uploader, _ = NewUploaderEntity(c)
		session.Set("uploader", uploader.Uuid)
		session.Save()
	}

	return uploader
}
