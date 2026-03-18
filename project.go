package main

import (
	"fmt"
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

func NewProjectEntity(uploader TUploaderEntity, title string) (TProjectEntity, error) {
	currentTime := utils.GetCurrentFormattedTime()
	titleOrTime := title
	if title == "" {
		titleOrTime = currentTime
	}

	existingProject := TProjectEntity{}
	row := dbwrapper.DB.QueryRow("SELECT * FROM projects WHERE uploader_id = ? AND title = ?", uploader.Id, title)
	err := existingProject.ScanFullRow(row)
	if err == nil {
		return TProjectEntity{}, fmt.Errorf(`Project %v already exists`, existingProject.Title)
	}

	return TProjectEntity{
		UploaderId: uploader.Id,
		Title:      titleOrTime,
		CreatedAt:  currentTime,
		UpdatedAt:  currentTime,
	}, nil
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

func (project *TProjectEntity) Delete() error {
	stmt, err := dbwrapper.DB.Prepare("DELETE FROM projects WHERE id = ?")
	if err == nil {
		_, err = stmt.Exec(project.Id)
	}
	return err
}

func (project *TProjectEntity) CreateFolderEntity(folderPath string) (TFolderEntity, error) {
	id := 0
	err := dbwrapper.DB.QueryRow("SELECT id FROM projects WHERE id = ?", project.Id).
		Scan(&id)
	if id == 0 {
		return TFolderEntity{}, fmt.Errorf("Can't create folder for unsaved project")
	}

	folderName := strings.TrimSpace(path.Base(folderPath))
	if !IsAcceptablePathName(folderName) && folderPath != "." {
		return TFolderEntity{}, fmt.Errorf("Invalid folder name %v", folderName)
	}

	existingFolder := TFolderEntity{}
	row := dbwrapper.DB.QueryRow("SELECT * FROM folders WHERE path = ?")
	err = existingFolder.ScanFullRow(row)
	if err == nil {
		return TFolderEntity{}, fmt.Errorf(`Folder %v already exists`, existingFolder.Path)
	}

	currentTime := utils.GetCurrentFormattedTime()

	return TFolderEntity{
		ProjectId: project.Id,
		Path:      folderPath,
		CreatedAt: currentTime,
		UpdatedAt: currentTime,
	}, nil
}

func (project *TProjectEntity) GetFoldersTree() ([]TFolder, error) {
	folders := []TFolder{}

	rows, err := dbwrapper.DB.Query(`
		SELECT
			folders.id,
			folders.path,
			images.id,
			images.url,
			images.extension,
			images.filename,
			images.size_bytes,
			images.width,
			images.height,
			images.created_at,
			images.updated_at
		FROM folders
		LEFT JOIN images ON images.folder_id = folders.id
		WHERE folders.project_id = ?
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
			&img.Url,
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

	return OrganizeFolders(folders), nil
}

func GetFolderDepth(f string) int {
	if f == "." {
		return 0
	}

	return len(strings.Split(f, "/"))
}

func OrganizeFolders(folders []TFolder) []TFolder {
	if len(folders) < 1 {
		return nil
	}

	flat := make([]TFolder, len(folders))
	copy(flat, folders)

	// сортировка: "." - всегда с индексом 0
	// дальше по иерархии: папки верхнего уровня с меньшим индексом, вложенные папки - с большим
	slices.SortStableFunc(flat, func(a, b TFolder) int {
		da, db := GetFolderDepth(a.Path), GetFolderDepth(b.Path)
		if da != db {
			return da - db
		}

		return strings.Compare(a.Path, b.Path)
	})

	// flat, перенесённый в map, где ключами выступают [TFolder.Path]
	nodes := make(map[string]TFolder, len(flat))
	// ключ - [TFolder.Path] верхнего уровня, значение - список непосредственно вложенных [TFolder.Path]
	children := make(map[string][]string, len(flat))
	// список корневых [TFolder.Path], непосредственные потомки "."
	roots := make([]string, 0, len(flat))

	// наполнение [nodes]
	for _, f := range flat {
		nodes[f.Path] = f
	}

	// наполнение [children] и [roots]
	for _, f := range flat {
		if f.Path == "." {
			continue
		}

		parentPath := path.Dir(f.Path)
		if _, ok := nodes[parentPath]; ok && parentPath != "." {
			children[parentPath] = append(children[parentPath], f.Path)
		} else {
			roots = append(roots, f.Path)
		}
	}

	// на основе пути [p], получить [nodes[p]] и рекурсивно заполнить вложенные папки [nodes[p].Folders]
	var build func(p string) TFolder
	build = func(p string) TFolder {
		node := nodes[p]
		node.Folders = make([]TFolder, 0, len(children[p]))
		for _, c := range children[p] {
			node.Folders = append(node.Folders, build(c))
		}
		return node
	}

	if _, ok := nodes["."]; ok {
		r := build(".")
		for _, rootPath := range roots {
			r.Folders = append(r.Folders, build(rootPath))
		}
		return []TFolder{r}
	}

	out := make([]TFolder, 0, len(roots))
	for _, rootPath := range roots {
		out = append(out, build(rootPath))
	}
	return out
}

// краткая информация о проекте (используется в списках)
type TProjectPreview struct {
	Id            int                    `json:"id"`
	Title         string                 `json:"title"`
	CreatedAt     string                 `json:"created_at"`
	UpdatedAt     string                 `json:"updated_at"`
	Optimizations []TOptimizationPreview `json:"optimizations"`
}
