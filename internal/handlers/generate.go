package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	"diamond-mosaic/internal/db"
	"diamond-mosaic/internal/image"
	"diamond-mosaic/internal/pdf"
)

// Palette — глобальная палитра из БД.
var Palette []db.PaletteColor

func SetPaletteFromDB(p []db.PaletteColor) {
	Palette = p
}

// GenerateHandler отдаёт PDF с мозаикой (сверху) и легендой, разбитой на страницы по необходимости.
func GenerateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	widthStr := r.FormValue("width")
	heightStr := r.FormValue("height")
	if widthStr == "" || heightStr == "" {
		http.Error(w, "Не указаны размеры", http.StatusBadRequest)
		return
	}
	widthCm, err := strconv.Atoi(widthStr)
	heightCm, err := strconv.Atoi(heightStr)
	if err != nil || widthCm <= 0 || heightCm <= 0 || widthCm > 2000 || heightCm > 2000 {
		http.Error(w, "Некорректные размеры", http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Ошибка получения файла", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Получаем мозаичное изображение и статистику использования цветов
	mosaicImg, usages, sizeInfo, err := image.Process(file, Palette, widthCm, heightCm)
	if err != nil {
		log.Printf("Ошибка обработки изображения: %v", err)
		http.Error(w, fmt.Sprintf("Ошибка обработки изображения: %v", err), http.StatusInternalServerError)
		return
	}

	// Генерируем PDF через отдельную функцию
	pdfBytes, err := pdf.GeneratePDF(mosaicImg, usages, sizeInfo)
	if err != nil {
		log.Printf("Ошибка формирования PDF: %v", err)
		http.Error(w, "Ошибка формирования PDF", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="mosaic_with_legend.pdf"`)
	if _, err := w.Write(pdfBytes); err != nil {
		log.Printf("Ошибка записи ответа: %v", err)
	}
}
