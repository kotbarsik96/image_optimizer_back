package main

type Extension int

const (
	PNG Extension = iota
	JPEG
	WEBP
	AVIF
	SVG
)

type ProjectEnvMode int

const (
	ENVMODE_DEV ProjectEnvMode = iota
	ENVMODE_PROD
)

type FileKind int

const (
	Folder FileKind = iota
	File
)

// запрос на создание нового проекта
type TNewProjectRequest struct {
	// список изображений (может быть пустым)
	images []any
	// название проекта (может быть не указано - в таком случае названием служит дата создания проекта)
	title string
}

// краткая информация об оптимизации (используется в списках)
type TOptimizationPreview struct {
	Id              int    `json:"id"`
	OutputExtension string `json:"output_extension"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}
