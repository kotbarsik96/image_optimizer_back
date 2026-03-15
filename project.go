package main

import "image_optimizer/imgopt_db"

// полная информация о проекте
type TProjectEntity struct {
	Id int `json:"id"`
	// id создателя проекта
	UploaderId int    `json:"uploader_id"`
	Title      string `json:"title"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

func NewProjectEntity(uploader TUploaderEntity, title string) TProjectEntity {
	currentTime := utils.GetCurrentFormattedTime()
	t := title
	if title == "" {
		t = currentTime
	}

	return TProjectEntity{
		UploaderId: uploader.Id,
		Title:      t,
		CreatedAt:  currentTime,
		UpdatedAt:  currentTime,
	}
}

func GetProjectEntity(id int) (TProjectEntity, error) {
	entity := TProjectEntity{}
	stmt := dbwrapper.DB.QueryRow("SELECT * FROM projects WHERE id = ?", id)
	err := stmt.Scan(&entity.Id, &entity.UploaderId, &entity.Title, &entity.CreatedAt, &entity.UpdatedAt)
	return entity, err
}

func (project *TProjectEntity) ScanFullRow(row imgopt_db.DatabaseRow) error {
	return row.Scan(&project.Id,
		&project.UploaderId,
		&project.Title,
		&project.CreatedAt,
		&project.UpdatedAt)
}

func (project *TProjectEntity) Save() (int, error) {
	id, err := dbwrapper.SaveEntity("projects", project)
	project.Id = id
	return id, err
}

func (project *TProjectEntity) GetUploader() (TUploaderEntity, error) {
	var uploader TUploaderEntity
	stmt := dbwrapper.DB.QueryRow("SELECT * FROM uploaders WHERE id = ?", project.UploaderId)
	err := stmt.Scan(&uploader.Id, &uploader.Uuid)
	return uploader, err
}

func (project *TProjectEntity) GetFolder(path string) (TFolder, error) {
	folder := TFolder{}

	rows, err := dbwrapper.DB.Query(`
		SELECT 
			folders.path, 
			images.id, 
			images.extension, 
			images.filename, 
			images.size_bytes
			images.width,
			images.height,
			images.created_at,
			images.updated_at
		FROM folders
		WHERE folders.id IN (
			SELECT folder_id FROM projects_folders 
			WHERE project_id = ?
		)
		JOIN images ON images.folder_id = folders.id
	`, project.Id)

	if err != nil {
		return folder, err
	}
	defer rows.Close()

	for rows.Next() {

	}

	return folder, err
}

// краткая информация о проекте (используется в списках)
type TProjectPreview struct {
	Id            int
	Title         string
	CreatedAt     string
	UpdatedAt     string
	Optimizations []TOptimizationPreview
}
