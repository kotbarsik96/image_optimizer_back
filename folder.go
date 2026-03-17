package main

import (
	_ "image/jpeg"
	_ "image/png"
	"image_optimizer/imgopt_db"
	"regexp"
	"strings"

	_ "golang.org/x/image/webp"
)

var foldernameRegExp = regexp.MustCompile(`^[\pL\pM\pN._ -]+$`)

type TFolderEntity struct {
	Id             int    `json:"id"`
	ProjectId      int    `json:"project_id"`
	OptimizationId int    `json:"optimization_id"`
	Path           string `json:"path"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

type TFolder struct {
	Id      int            `json:"id"`
	Path    string         `json:"path"`
	Folders []TFolder      `json:"folders"`
	Images  []TImageEntity `json:"images"`
}

func GetFolderEntity(id int) (TFolderEntity, error) {
	folder := TFolderEntity{}
	row := dbwrapper.DB.QueryRow("SELECT * FROM folders WHERE id = ?", id)
	err := folder.ScanFullRow(row)
	return folder, err
}

func (folder *TFolderEntity) Save() error {
	folder.UpdatedAt = utils.GetCurrentFormattedTime()
	id, err := dbwrapper.SaveEntity("folders", folder)
	folder.Id = id
	return err
}

func (folder *TFolderEntity) ScanFullRow(row imgopt_db.DatabaseRow) error {
	return row.Scan(&folder.Id,
		&folder.ProjectId,
		&folder.OptimizationId,
		&folder.Path,
		&folder.CreatedAt,
		&folder.UpdatedAt)
}

func IsAcceptableFolderName(name string) bool {
	if !foldernameRegExp.MatchString(name) {
		return false
	}

	if name == "." || name == ".." {
		return false
	}

	if strings.HasSuffix(name, ".") || strings.HasSuffix(name, " ") {
		return false
	}

	base := strings.ToUpper(name)
	if i := strings.IndexByte(base, '.'); i >= 0 {
		base = base[:i]
	}

	switch base {
	case "CON", "PRN", "AUX", "NUL",
		"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
		"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9":
		return false
	}

	return true
}

// файл с общей информацией
type TFileEntity struct {
	Id        int    `json:"id"`
	FolderId  int    `json:"folder_id"`
	Extension string `json:"extension"`
	Filename  string `json:"filename"`
	SizeBytes int    `json:"size_bytes"`
}
