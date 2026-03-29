package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

var MinOptimizationPercent = 10
var MaxOptimizationPercent = 100

var ESupportedOptimizationExtensions = []string{
	"png",
	"webp",
	"jpg",
	"avif",
}

type Optimization struct {
	ID         uint      `gorm:"primarykey" json:"id"`
	ProjectID  uint      `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"project_id"`
	Title      string    `json:"title"`
	Extensions string    `json:"extensions"` // расширения в виде "png|webp|avif" или "png"
	Sizes      string    `json:"sizes"`      // размеры в процентах: для "25|50|75" будут созданы 3 версии изображения; каждое из указанных чисел не может быть меньше MinOptimizationPercent и больше MaxOptimizationPercent
	CreatedAt  time.Time `json:"created_at,omitzero"`
	UpdatedAt  time.Time `json:"updated_at,omitzero"`
}

// получить []string из строки вида "avif|jpeg|png": []string{"avif", "jpeg", "png"}
func GetOptimizationExtensions(extensionsRaw string) ([]string, error) {
	extensions := []string{}
	for ext := range strings.SplitSeq(extensionsRaw, "|") {
		if !slices.Contains(ESupportedOptimizationExtensions, ext) {
			return nil, fmt.Errorf("%v: %w", ext, ErrNotSupportedExtension)
		}
		extensions = append(extensions, ext)
	}

	return extensions, nil
}

// получить []int из строки вида "75|50|100". Слайс будет отсортирован по возрастанию: []int{50, 75, 100}
func GetOptimizationSizes(sizesRaw string) ([]int, error) {
	sizes := []int{}

	for size := range strings.SplitSeq(sizesRaw, "|") {
		sizeInt, err := strconv.Atoi(size)
		if err != nil {
			return nil, fmt.Errorf("value %v: %w", size, ErrInvalidInt)
		}

		if sizeInt < MinOptimizationPercent {
			return nil, fmt.Errorf("%w (min: %v, %v given)", ErrLessThanMin, MinOptimizationPercent, size)
		}
		if sizeInt > MaxOptimizationPercent {
			return nil, fmt.Errorf("%w (max: %v, %v given)", ErrMoreThanMax, MaxOptimizationPercent, size)
		}

		sizes = append(sizes, sizeInt)
	}

	slices.SortStableFunc(sizes, func(a, b int) int {
		return a - b
	})

	return sizes, nil
}

func (optimization *Optimization) Delete(ctx context.Context) error {
	_, err := gorm.G[Optimization](gormDb).Where("id = ?", optimization.ID).Delete(ctx)
	return err
}

/*
Запуск оптимизации

Формируется путь OPT_PATH: /[RESOURCES_PATH]/uploaders/[UPLOADER_UUID]/optimizations/[OPTIMIZATION_NAME]

Внутри OPT_PATH формируются:

1. OPT_PATH/temp/archive - папка для создания архива

2. OPT_PATH/temp/downloads - папка, в которую будут скачаны оригиналы изображений, подлежащие оптимизации

Когда процесс оптимизации завершится, всё содержимое OPT_PATH/temp/archive будет архивировано в OPT_PATH/[OPTIMIZATION_NAME].zip. Вся папка temp будет удалена
*/
func (o *Optimization) Start() {
	// подготовка: создание необходимых папок
	ctx := context.Background()
	project, err := gorm.G[Project](gormDb).Where("id = ?", o.ProjectID).First(ctx)
	if err != nil {
		log.Fatalf("project not found for opt %v: %v\n", o.ID, err)
	}

	uploader, err := gorm.G[Uploader](gormDb).Where("id = ?", project.UploaderID).First(ctx)
	if err != nil {
		log.Fatalf("uploader not found for project %v: %v\n", project.ID, err)
	}

	log.Printf("Optimization %v started\n", o.Title)

	storageLocal := Storages[EStorageLocal].(StorageLocal)
	// [OPT_PATH]
	optPath := path.Join(storageLocal.RootPath, uploader.GetOptimizationsPath(), o.Title)

	tempDir := o.CreateDirFatal(optPath, "temp")
	archiveDir := o.CreateDirFatal(tempDir, "archive")
	downloadsDir := o.CreateDirFatal(tempDir, "downloads")

	rootFolder, err := project.GetRootFolder(ctx)
	if err != nil {
		log.Fatalf("Could not get root folder for optimization %v: %v", o.Title, err)
	}

	// подготовка: инициализация прогресса
	progress := o.NewOptimizationProgress(ctx, project)

	// подготовка завершена: запуск оптимизации с корневой папки
	rootFolder.OptimizeImages(ctx, *o, archiveDir, downloadsDir, progress)

	log.Printf("Optimization %v done\n", o.Title)

	log.Printf("Archiving optimization %v to zip file\n", o.Title)

	// файлы оптмизированы: сформировать архив
	zipPath := path.Join(optPath, o.Title+".zip")
	err = ZipDir(archiveDir, zipPath)
	ProgressesStorage.FinishProgress(progress)

	if err != nil {
		log.Printf("Could not create zip archive for optimization %v: %v", o.Title, err)
	} else {
		log.Printf("Zip archive created for optimization %v", zipPath)
	}

	// удаление /temp
	// time.Sleep нужен для снижения рисков ошибки "The file is being used by another process"
	time.Sleep(time.Millisecond * 1500)
	err = os.RemoveAll(tempDir)
	if err != nil {
		log.Printf("Could not remove temporary dir %v: %v", optPath, err)
	}
}

func (o *Optimization) CreateDirFatal(rootPath, dirName string) string {
	outputDir := path.Join(rootPath, dirName)
	err := os.MkdirAll(outputDir, 0666)
	if err != nil {
		log.Fatalf("Could not create %v directory %v: %v", dirName, outputDir, err)
	}
	return outputDir
}

func (o *Optimization) NewOptimizationProgress(ctx context.Context, project Project) *Progress {
	imagesCount, err := gorm.G[Image](gormDb).Where(`folder_id IN (
		SELECT id FROM folders WHERE project_id = ?
	)`, project.ID).Count(ctx, "id")
	if err != nil {
		log.Printf("Progress watching for optimization %v started incorrectly: could not get images count: %v", o.Title, err)
	}

	// imagesCount + операция по созданию архива
	total := uint(imagesCount + 1)
	return ProgressesStorage.NewProgress(EProgressStorageOptimizations, o.ID, total)
}
