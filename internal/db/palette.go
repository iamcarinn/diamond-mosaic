package db

import (
	"database/sql"
	"fmt"

	"github.com/lucasb-eyer/go-colorful"
	_ "github.com/lib/pq"
)

// PaletteColor описывает цвет из палитры.
type PaletteColor struct {
	DMCCode string			// Код цвета по DMC
	Name    string			// Название цвета
	Color   colorful.Color	// Цвет в RGB
	Symbol  string			// Символ для схемы
}

// LoadPalette подключается к базе данных и загружает палитру цветов из таблицы palette.
// На выходе — срез PaletteColor, каждый элемент содержит код, название и цвет (RGB).
func LoadPalette(connStr string) ([]PaletteColor, error) {
	// 1. Открываем соединение с БД
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("ошибка открытия БД: %w", err)
	}
	defer db.Close()

	// 2. Делаем SELECT-запрос к таблице palette
	rows, err := db.Query("SELECT dmc_code, name, r, g, b FROM palette")
	if err != nil {
		return nil, fmt.Errorf("ошибка запроса к таблице palette: %w", err)
	}
	defer rows.Close()

	// 3. Считываем строки и строим срез PaletteColor
	var palette []PaletteColor
	for rows.Next() {
		var dmcCode, name string
		var r, g, b int
		if err := rows.Scan(&dmcCode, &name, &r, &g, &b); err != nil {
			return nil, fmt.Errorf("ошибка чтения строки: %w", err)
		}
		color := colorful.Color{
			R: float64(r) / 255.0,
			G: float64(g) / 255.0,
			B: float64(b) / 255.0,
		}
		palette = append(palette, PaletteColor{
			DMCCode: dmcCode,
			Name:    name,
			Color:   color,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка после чтения строк: %w", err)
	}
	// 5. Возвращаем палитру
	return palette, nil
}

// FilterPalette оставляет только "достаточно разные" цвета из исходной палитры.
// minDist — минимальная дистанция между цветами в пространстве Lab.
func FilterPalette(palette []PaletteColor, minDist float64) []PaletteColor {
    var filtered []PaletteColor

	// 1. Перебираем все цвета из палитры
    for _, pc := range palette {
        tooClose := false
        l1, a1, b1 := pc.Color.Lab()

		// 2. Проверяем, что этот цвет не слишком близок к уже отобранным
        for _, fpc := range filtered {
            l2, a2, b2 := fpc.Color.Lab()
            dl := l1 - l2
            da := a1 - a2
            db_ := b1 - b2
            dist := dl*dl + da*da + db_*db_
            if dist < minDist*minDist {
                tooClose = true
                break
            }
        }
		// 3. Если цвет уникальный по расстоянию — добавляем
        if !tooClose {
            filtered = append(filtered, pc)
        }
    }
	// 4. Возвращаем отфильтрованный срез
    return filtered
}
