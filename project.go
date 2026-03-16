package main

import (
	"image_optimizer/imgopt_db"
	"path"
	"slices"
	"strings"
)

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

func (project *TProjectEntity) GetFoldersTree() ([]TFolder, error) {
	folders := []TFolder{}

	rows, err := dbwrapper.DB.Query(`
		SELECT
			folders.id,
			folders.path,
			images.id,
			images.extension,
			images.filename,
			images.size_bytes,
			images.width,
			images.height,
			images.created_at,
			images.updated_at
		FROM folders
		LEFT JOIN images ON images.folder_id = folders.id
		WHERE folders.id IN (
			SELECT folder_id FROM projects_folders
			WHERE project_id = ?
		)
		ORDER BY folders.path
	`, project.Id)
	if err != nil {
		return folders, err
	}
	defer rows.Close()

	folder := TFolder{}
	for rows.Next() {
		var id int
		var path string
		img := TImageEntity{}
		rows.Scan(
			&id,
			&path,
			&img.Id,
			&img.Extension,
			&img.Filename,
			&img.SizeBytes,
			&img.Width,
			&img.Height,
			&img.CreatedAt,
			&img.UpdatedAt,
		)

		isNextFolder := folder.Path != path
		if isNextFolder {
			folder = TFolder{
				Id:      id,
				Path:    path,
				Folders: []TFolder{},
				Images:  []TImageEntity{},
			}
		}

		if img.Id != 0 {
			folder.Images = append(folder.Images, img)
		}

		if isNextFolder {
			folders = append(folders, folder)
		}
	}

	folders = OrganizeFolders(folders)

	return folders, nil
}

func OrganizeFolders(folders []TFolder) []TFolder {
	organized := []TFolder{}

	var hasRoot bool
	for _, f := range folders {
		if f.Path == "." {
			hasRoot = true
			break
		}
	}

	if hasRoot {
		slices.SortFunc(folders, func(a, b TFolder) int {
			if a.Path == "." {
				return -1
			}
			if b.Path == "." {
				return 1
			}

			len1 := len(strings.Split(a.Path, "/"))
			len2 := len(strings.Split(b.Path, "/"))
			return len1 - len2
		})

		for i, f := range folders {
			if f.Path == "." {
				if i == 0 {
					break
				}

				folders[0], folders[i] = folders[i], folders[0]
			}
		}

		folders[0].Folders = OrganizeFolders(folders[1:])

		organized = append(organized, folders[0])
	} else {
		for i := len(folders) - 1; i > 0; i-- {
		l:
			for j := 0; j < i; j++ {
				isParent := folders[j].Path == path.Dir(folders[i].Path)
				if isParent {
					folders[j].Folders = append(folders[j].Folders, folders[i])
					folders = slices.Delete(folders, i, i+1)
					break l
				}
			}
		}
		organized = folders
	}

	return organized
}

// краткая информация о проекте (используется в списках)
type TProjectPreview struct {
	Id            int
	Title         string
	CreatedAt     string
	UpdatedAt     string
	Optimizations []TOptimizationPreview
}
