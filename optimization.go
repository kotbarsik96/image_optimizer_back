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
	Folders    []Folder  `json:"folders,omitzero"`
	RootFolder Folder    `gorm:"-" json:"root_folder,omitzero"`
	CreatedAt  time.Time `json:"created_at,omitzero"`
	UpdatedAt  time.Time `json:"updated_at,omitzero"`
}

func (opt *Optimization) GetRootFolder(ctx context.Context) (Folder, error) {
	return gorm.G[Folder](gormDb).
		Where("optimization_id = ? AND path = '.'", opt.ID).
		Preload("Nested", nil).
		Preload("Images", nil).
		First(ctx)
}

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

	return sizes, nil
}

// удалить оптимизацию и корневую папку. Удалит все связанные с оптимизацией папки и изображения
func (optimization *Optimization) Delete(ctx context.Context) error {
	rootFolder, err := optimization.GetRootFolder(ctx)
	if err != nil {
		log.Printf("Optimization %v's root folder not found: %v", optimization.Title, err)
	}

	err = rootFolder.DeleteEvenIfRoot(ctx)
	if err != nil {
		log.Printf("Could not delete optimization %v's root folder: %v", optimization.Title, err)
	}

	_, err = gorm.G[Optimization](gormDb).Where("id = ?", optimization.ID).Delete(ctx)

	return err
}

func (opt *Optimization) Start() {
	ctx := context.Background()
	project, err := gorm.G[Project](gormDb).Where("id = ?", opt.ProjectID).First(ctx)
	if err != nil {
		log.Fatalf("project not found for opt %v: %v\n", opt.ID, err)
	}

	uploader, err := gorm.G[Uploader](gormDb).Where("id = ?", project.UploaderID).First(ctx)
	if err != nil {
		log.Fatalf("uploader not found for project %v: %v\n", project.ID, err)
	}

	log.Printf("Optimization %v started: %v\n", opt.Title, time.Now().Format(time.TimeOnly))

	dirname := path.Join("_optimizations", uploader.Uuid, opt.Title)
	err = os.MkdirAll(dirname, 0666)
	if err != nil {
		log.Fatalf("Could not create directory %v: %v", dirname, err)
	}

	rootFolder, err := project.GetRootFolder(ctx)
	if err != nil {
		log.Fatalf("Could not get root folder for optimization %v: %v", opt.Title, err)
	}

	rootFolder.OptimizeImages(ctx, *opt, dirname)

	log.Printf("Optimization %v done: %v\n", opt.Title, time.Now().Format(time.TimeOnly))
}
