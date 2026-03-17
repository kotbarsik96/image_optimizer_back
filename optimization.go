package main

// краткая информация об оптимизации (используется в списках)
type TOptimizationPreview struct {
	Id              int    `json:"id"`
	OutputExtension string `json:"output_extension"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}
