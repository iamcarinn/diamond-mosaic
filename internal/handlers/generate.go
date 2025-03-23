package handlers

import (
	"diamond-mosaic/internal/image"
	"diamond-mosaic/internal/db"
	//"encoding/json"
	"fmt"
	"image/png"
	"log"
	"net/http"
)

// palette глобальная переменная для хранения палитры, загруженной из БД.
var palette []interface{} // временно, чтобы можно было использовать интерфейс, заменим на конкретный тип ниже

// SetPalette устанавливает глобальную палитру для обработки.
func SetPalette(p interface{}) {
	palette = p.([]interface{})
}

// Для более корректного использования типа, можно объявить глобальную переменную так:
var GlobalPalette []interface{}

func SetGlobalPalette(p []interface{}) {
	GlobalPalette = p
}

// Перепишем глобальную переменную:
var Palette []db.PaletteColor

// SetPalette задаёт глобальную палитру.
func SetPaletteFromDB(p []db.PaletteColor) {
	Palette = p
}

// GenerateHandler обрабатывает POST-запрос на генерацию схемы.
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

	// Вызываем обработку изображения с передачей палитры.
	img, err := image.Process(file, Palette)
	if err != nil {
		log.Printf("Ошибка обработки изображения: %v", err)
		http.Error(w, fmt.Sprintf("Ошибка обработки изображения: %v", err), http.StatusInternalServerError)
		return
	}

	// Отдаём результат в формате PNG
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Disposition", "attachment; filename=\"mosaic.png\"")
	if err := png.Encode(w, img); err != nil {
		http.Error(w, "Ошибка кодирования PNG", http.StatusInternalServerError)
		return
	}
}
