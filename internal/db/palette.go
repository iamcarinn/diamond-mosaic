package db

import (
	"database/sql"
	"fmt"

	"github.com/lucasb-eyer/go-colorful"
	_ "github.com/lib/pq"
)

// PaletteColor описывает цвет из палитры.
type PaletteColor struct {
	DMCCode string
	Name    string
	Color   colorful.Color
	Symbol  string
}

// LoadPalette подключается к БД и загружает данные из таблицы palette.
func LoadPalette(connStr string) ([]PaletteColor, error) {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("ошибка открытия БД: %w", err)
	}
	defer db.Close()

	rows, err := db.Query("SELECT dmc_code, name, r, g, b FROM palette")
	if err != nil {
		return nil, fmt.Errorf("ошибка запроса к таблице palette: %w", err)
	}
	defer rows.Close()

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
	return palette, nil
}

// FilterPalette — оставить только "достаточно разные" цвета
func FilterPalette(palette []PaletteColor, minDist float64) []PaletteColor {
    var filtered []PaletteColor

    for _, pc := range palette {
        tooClose := false
        l1, a1, b1 := pc.Color.Lab()
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
        if !tooClose {
            filtered = append(filtered, pc)
        }
    }
    return filtered
}
