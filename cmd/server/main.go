package main

import (
	"log"
	"net/http"
	"path/filepath"
	"diamond-mosaic/internal/handlers"
)

func main() {
	// Раздача статики
	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/", fs)

	// Обработчик генерации
	http.HandleFunc("/generate", handlers.GenerateHandler)

	log.Println("Сервер запущен на http://localhost:8080")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal(err)
	}
}
