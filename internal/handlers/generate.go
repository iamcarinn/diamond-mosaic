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

// Palette — глобальная палитра из базы данных DMC, используется всеми обработчиками для подбора цветов.
var Palette []db.PaletteColor

// SetPaletteFromDB устанавливает глобальную палитру для использования в обработчиках.
func SetPaletteFromDB(p []db.PaletteColor) {
	Palette = p
}

// GenerateHandler обрабатывает POST-запрос /generate и возвращает PDF-файл с мозаикой и легендой.
func GenerateHandler(w http.ResponseWriter, r *http.Request) {
	// 1. Разрешён только POST-запрос
	if r.Method != http.MethodPost {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	// 2. Получаем размеры основы из формы
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

	// 3. Получаем загруженный PNG-файл
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Ошибка получения файла", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// 4. Обрабатываем изображение: ресайз, подбор цветов, статистика
	mosaicImg, usages, sizeInfo, err := image.Process(file, Palette, widthCm, heightCm)
	if err != nil {
		log.Printf("Ошибка обработки изображения: %v", err)
		http.Error(w, fmt.Sprintf("Ошибка обработки изображения: %v", err), http.StatusInternalServerError)
		return
	}

	// 5. Генерируем PDF-файл по результатам обработки
	pdfBytes, err := pdf.GeneratePDF(mosaicImg, usages, sizeInfo)
	if err != nil {
		log.Printf("Ошибка формирования PDF: %v", err)
		http.Error(w, "Ошибка формирования PDF", http.StatusInternalServerError)
		return
	}

	// 6. Отправляем PDF-файл на скачивание
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="mosaic_with_legend.pdf"`)
	if _, err := w.Write(pdfBytes); err != nil {
		log.Printf("Ошибка записи ответа: %v", err)
	}
}
