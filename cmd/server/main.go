package main

import (
	"log"
	"net/http"

	"diamond-mosaic/internal/db"
	"diamond-mosaic/internal/handlers"
)

// main инициализирует подключение к базе данных, загружает палитру,
// настраивает маршруты и запускает HTTP-сервер приложения.
func main() {
	// 1. Задаём строку подключения к PostgreSQL
	connStr := "postgres://postgres:password@localhost:5432/diamond_mosaic?sslmode=disable"

	// 2. Загружаем палитру цветов из базы данных
	palette, err := db.LoadPalette(connStr)
	//  обрезаем палитру, оставляя только необходимые цвета (порог 0.11)
	palette = db.FilterPalette(palette, 0.11)
	if err != nil {
		log.Fatalf("Ошибка загрузки палитры: %v", err)
	}
	log.Printf("Загружено цветов: %d", len(palette))

	// 3. Передаём палитру в обработчики (глобально для текущего прототипа)
	handlers.SetPaletteFromDB(palette)

	// 4. Раздаём статику (HTML, CSS, JS) по адресу /
	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/", fs)

	// 5. Добавляем обработчик генерации схемы (POST /generate)
	http.HandleFunc("/generate", handlers.GenerateHandler)

	// 6. Раздаём всё содержимое папки static по адресу /static/
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// 7. Запускаем HTTP-сервер
	log.Println("Сервер запущен на http://localhost:8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
