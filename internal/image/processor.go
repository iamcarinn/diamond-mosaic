package image

import (
	"image"
	"image/color"
	"image/draw"
	"io"
	"log"

	"github.com/disintegration/imaging"
)

func Process(file io.Reader) (image.Image, error) {
	// Загружаем изображение
	src, err := imaging.Decode(file)
	if err != nil {
		return nil, err
	}

	// Масштабируем до фиксированного размера (например, 100x100)
	resized := imaging.Resize(src, 100, 100, imaging.NearestNeighbor)

	// Рисуем сетку: каждая ячейка будет 10x10 пикселей
	cellSize := 10
	gridW := resized.Bounds().Dx()
	gridH := resized.Bounds().Dy()

	dst := image.NewRGBA(image.Rect(0, 0, gridW*cellSize, gridH*cellSize))

	for y := 0; y < gridH; y++ {
		for x := 0; x < gridW; x++ {
			c := resized.At(x, y)
			rect := image.Rect(x*cellSize, y*cellSize, (x+1)*cellSize, (y+1)*cellSize)
			draw.Draw(dst, rect, &image.Uniform{c}, image.Point{}, draw.Src)
		}
	}

	log.Println("PNG схема успешно сгенерирована")
	return dst, nil
}
