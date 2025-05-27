package main

import (
	"log"
	"net/http"

	"diamond-mosaic/internal/db"
	"diamond-mosaic/internal/handlers"
)

func main() {
	// Параметры подключения к БД (отредактируй по своим настройкам)
	connStr := "postgres://postgres:password@localhost:5432/diamond_mosaic?sslmode=disable"

	// Загружаем палитру из базы данных
	palette, err := db.LoadPalette(connStr)
	palette = db.FilterPalette(palette, 0.11)

	if err != nil {
		log.Fatalf("Ошибка загрузки палитры: %v", err)
	}
	log.Printf("Загружено цветов: %d", len(palette))

	// Передаём палитру в обработчики (глобально для текущего прототипа)
	// handlers.SetPalette(palette)
	handlers.SetPaletteFromDB(palette)

	// Раздача статики
	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/", fs)

	// Обработчик генерации схемы
	http.HandleFunc("/generate", handlers.GenerateHandler)
	// Раздаём всё из static по адресу /static/
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	log.Println("Сервер запущен на http://localhost:8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
