package main

import (
	"image_optimizer/imgopt_db"

	"github.com/google/uuid"
)

type TUploaderEntity struct {
	Id   int    `json:"id"`
	Uuid string `json:"uuid"`
}

// получить данные об uploader'е из базы. Добавить в базу, если uuid пуст
func (uploader *TUploaderEntity) GetData() error {
	// попробовать получить из базы
	if uploader.Id == 0 && uploader.Uuid != "" {
		row := dbwrapper.DB.QueryRow("SELECT * FROM uploaders WHERE uuid = ?", uploader.Uuid)
		err := uploader.ScanFullRow(row)
		if err == nil {
			return nil
		}
	}

	// в базе ещё нет; добавить
	if uploader.Uuid == "" {
		uploader.Uuid = uuid.New().String()
	}
	err := uploader.Save()

	return err
}

func (uploader *TUploaderEntity) ScanFullRow(row imgopt_db.DatabaseRow) error {
	return row.Scan(&uploader.Id, &uploader.Uuid)
}

func (uploader *TUploaderEntity) Save() error {
	id, err := dbwrapper.SaveEntity("uploaders", uploader)
	uploader.Id = id
	return err
}
