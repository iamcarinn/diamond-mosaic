package handlers

import (
	"diamond-mosaic/internal/image"
	"fmt"
	"image/png"
	"net/http"
)

func GenerateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Не удалось получить файл", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Обработка изображения (заглушка)
	resultImg, err := image.Process(file)
	if err != nil {
		http.Error(w, fmt.Sprintf("Ошибка обработки: %v", err), http.StatusInternalServerError)
		return
	}

	// Заголовки для ответа
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Disposition", "attachment; filename=\"mosaic.png\"")

	// Отдаём PNG
	err = png.Encode(w, resultImg)
	if err != nil {
		http.Error(w, "Ошибка при создании PNG", http.StatusInternalServerError)
		return
	}
}
