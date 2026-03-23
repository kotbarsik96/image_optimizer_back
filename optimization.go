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

	slices.SortStableFunc(sizes, func(a, b int) int {
		return a - b
	})

	return sizes, nil
}

func (optimization *Optimization) Delete(ctx context.Context) error {
	_, err := gorm.G[Optimization](gormDb).Where("id = ?", optimization.ID).Delete(ctx)
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

	log.Printf("Optimization %v started\n", opt.Title)

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

	log.Printf("Optimization %v done\n", opt.Title)

	log.Printf("Archiving optimization %v to zip file\n", opt.Title)

	zipPath := path.Join("_optimizations", uploader.Uuid, opt.Title+".zip")
	err = ZipDir(dirname, zipPath)

	if err != nil {
		log.Printf("Could not create zip archive for optimization %v: %v", opt.Title, err)
	} else {
		log.Printf("Zip archive created for optimization %v", zipPath)
	}
}
