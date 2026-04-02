package main

import (
	"fmt"
	"os/exec"
	"path"
	"strconv"
)

func ResizeImage(inputPath, outputPath string, scale float64) error {
	scaleString := strconv.FormatFloat(scale, 'E', -1, 64)
	command := exec.Command("vips", "resize", inputPath, outputPath, scaleString)
	_, err := command.CombinedOutput()
	return err
}

func EncodeImageToExtension(inputPath, outputPath string) error {
	inputExt, _ := ToSupportedExtension(path.Ext(inputPath)[1:])
	outputExt, err := ToSupportedExtension(path.Ext(outputPath))
	if err != nil {
		return fmt.Errorf("%w: %v", err, outputExt)
	}

	if inputExt == outputExt {
		return CopyFile(outputPath, inputPath)
	}

	var command *exec.Cmd

	switch outputExt {
	case "avif":
		command = exec.Command("avifenc", "-q", "75", "-s", "3", inputPath, outputPath)
	case "webp":
		command = exec.Command("cwebp", "-q", "75", inputPath, "-o", outputPath)
	case "png":
	case "jpg":
		command = exec.Command("magick", "-quality", "50", inputPath, outputPath)
	default:
		return fmt.Errorf("%v: %w", outputExt, ErrNotSupportedExtension)
	}

	_, err = command.CombinedOutput()

	return err
}
