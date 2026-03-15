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
	id         int
	extension  Extension
	created_at string
}

// полная информация об оптимизации
type TOptimization struct {
	id         int
	project_id int
	// расширение, в которое будут преобразованы файлы
	output_extension Extension
	// размер в процентах, в который будут уменьшены файлы (должен быть <= 100%)
	output_size_percent int
}
