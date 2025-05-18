package handlers

import (
	"bytes"
	"diamond-mosaic/internal/db"
	"diamond-mosaic/internal/image"
	"fmt"
	"github.com/jung-kurt/gofpdf"
	"image/png"
	"log"
	"math"
	"net/http"
)

// Palette глобальная переменная для хранения палитры.
var Palette []db.PaletteColor

// SetPaletteFromDB устанавливает глобальную палитру для обработки.
func SetPaletteFromDB(p []db.PaletteColor) {
	Palette = p
}

// GenerateHandler обрабатывает POST-запрос на генерацию схемы и возвращает PDF с мозаикой.
func GenerateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Ошибка получения файла", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Обрабатываем изображение по заданной палитре
	img, err := image.Process(file, Palette)
	if err != nil {
		log.Printf("Ошибка обработки изображения: %v", err)
		http.Error(w, fmt.Sprintf("Ошибка обработки изображения: %v", err), http.StatusInternalServerError)
		return
	}

	// Кодируем картинку в временный PNG-буфер
	var imgBuf bytes.Buffer
	if err := png.Encode(&imgBuf, img); err != nil {
		http.Error(w, "Ошибка кодирования во временный PNG-буфер", http.StatusInternalServerError)
		return
	}

	// Создаём PDF (A4, мм) и вставляем картинку по центру
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()

	pdf.RegisterImageOptionsReader("mosaic", gofpdf.ImageOptions{
		ImageType: "PNG",
		ReadDpi:   false,
	}, &imgBuf)

	pageW, pageH := pdf.GetPageSize()
	info := pdf.GetImageInfo("mosaic")
	imgW := info.Width()
	imgH := info.Height()

	// Масштабируем до 90% от размеров страницы
	maxW := pageW * 0.9
	maxH := pageH * 0.9
	scale := math.Min(maxW/imgW, maxH/imgH)
	imgW *= scale
	imgH *= scale

	// Центрируем картинку
	x := (pageW - imgW) / 2
	y := (pageH - imgH) / 2

	pdf.ImageOptions("mosaic", x, y, imgW, imgH, false, gofpdf.ImageOptions{
		ImageType: "PNG",
		ReadDpi:   false,
	}, 0, "")

	// Генерируем PDF в буфер и отдаем клиенту
	var pdfBuf bytes.Buffer
	if err := pdf.Output(&pdfBuf); err != nil {
		http.Error(w, "Ошибка формирования PDF", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "attachment; filename=\"mosaic.pdf\"")
	if _, err := w.Write(pdfBuf.Bytes()); err != nil {
		log.Printf("Ошибка записи PDF в ответ: %v", err)
	}
}
