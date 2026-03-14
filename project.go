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

func (project *TProjectEntity) Save() (int, error) {
	id, err := dbwrapper.SaveEntity("projects", project)
	project.Id = id
	return id, err
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
