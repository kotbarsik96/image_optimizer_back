package main

// полная информация о проекте
type TProjectEntity struct {
	Id int `json:"id"`
	// id создателя проекта
	Uploader_id int    `json:"uploader_id"`
	Title       string `json:"title"`
	Created_at  string `json:"created_at"`
	Updated_at  string `json:"updated_at"`
}

func NewProjectEntity(uploader TUploaderEntity, title string) TProjectEntity {
	currentTime := utils.GetCurrentFormattedTime()
	t := title
	if title == "" {
		t = currentTime
	}

	return TProjectEntity{
		Uploader_id: uploader.Id,
		Title:       t,
		Created_at:  currentTime,
		Updated_at:  currentTime,
	}
}

func GetProjectEntity(id int) (TProjectEntity, error) {
	entity := TProjectEntity{}
	stmt := dbwrapper.DB.QueryRow("SELECT * FROM projects WHERE id = ?", id)
	err := stmt.Scan(&entity.Id, &entity.Uploader_id, &entity.Title, &entity.Created_at, &entity.Updated_at)
	return entity, err
}

func (project *TProjectEntity) Save() (int, error) {
	id, err := dbwrapper.SaveEntity("projects", project)
	project.Id = id
	return id, err
}

func (project *TProjectEntity) GetUploader() (TUploaderEntity, error) {
	var uploader TUploaderEntity
	stmt := dbwrapper.DB.QueryRow("SELECT * FROM uploaders WHERE id = ?", project.Uploader_id)
	err := stmt.Scan(&uploader.Id, &uploader.Uuid)
	return uploader, err
}

// краткая информация о проекте (используется в списках)
type TProjectPreview struct {
	Id            int
	Title         string
	Created_at    string
	Updated_at    string
	Optimizations []TOptimizationPreview
}

// ответ на запрос списка проектов
type TProjectResponse struct {
	TProjectEntity
	// сформированная при ответе файловая система
	Root_folder []TFolder
}
