package image

import (
	"diamond-mosaic/internal/db"
	"image"
	"image/color"
	"image/draw"
	"io"
	"io/ioutil"
	"log"

	// "golang.org/x/image/font"

	"github.com/disintegration/imaging"
	"github.com/golang/freetype"
	"github.com/lucasb-eyer/go-colorful"
)

// Символы для легенды
var allSymbols = []string{
	"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N", "O", "P", "Q", "R", "S", "T", "U", "V", "W", "X", "Y", "Z",
	"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o", "p", "q", "r", "s", "t", "u", "v", "w", "x", "y", "z",
	"0", "1", "2", "3", "4", "5", "6", "7", "8", "9",
	"!", "@", "#", "$", "%", "^", "&", "*", "(", ")", "-", "_", "+", "=", "~", "`",
	"[", "]", "{", "}", "<", ">", "?", "/", "\\", "|", ".", ",", ":", ";", "'", "\"",
	// "Ж", "З", "И", "Й", "Л", "П", "Ф", "Ц", "Ч", "Ш", "Щ", "Э", "Ю", "Я",
	// "ж", "з", "и", "й", "л", "п", "ф", "ц", "ч", "ш", "щ", "э", "ю", "я",
}

// // Присваиваем символы только используемым цветам
// for i := 0; i < len(usages) && i < len(allSymbols); i++ {
// 	usages[i].PaletteColor.Symbol = allSymbols[i]
// }

// ColorUsage связывает цвет из палитры с количеством пикселей (алмазов).
type ColorUsage struct {
	PaletteColor db.PaletteColor
	Count        int
}

// Process декодирует входное изображение, превращает его в мозаичный рисунок
// и собирает список уникальных DMC-цветов с их количеством использования.
func Process(file io.Reader, palette []db.PaletteColor, widthCm int, heightCm int) (image.Image, []ColorUsage, error) {
	// 1. Декодируем
	src, err := imaging.Decode(file)
	if err != nil {
		return nil, nil, err
	}

	// 2. Фильтр
	filtered := MedianFilter(src, 3)

	// Переводим см в мм
	widthMm := float64(widthCm) * 10.0
	heightMm := float64(heightCm) * 10.0

	strazMm := 2.5 // или бери из интерфейса, если вдруг меняется

	gridW := int(widthMm / strazMm)
	gridH := int(heightMm / strazMm)

	// 3. Масштабирование
	//const gridW, gridH = 50, 50 // TODO: возможность выбора
	resized := imaging.Resize(filtered, gridW, gridH, imaging.CatmullRom)

	// 4. Подбор ближайших цветов
	matched := MatchToPalette(resized, palette, gridW, gridH)
	AssignSymbolsToMatched(matched, allSymbols)

	// 5. Построение растрового холста и подсчёт цветов
	const cellSize = 10
	mosaic, usages := RenderMosaic(matched, cellSize)

	rgbaImg, ok := mosaic.(*image.RGBA)
	if !ok {
		// если вдруг другой тип, то сконвертируй:
		bounds := mosaic.Bounds()
		tmp := image.NewRGBA(bounds)
		draw.Draw(tmp, bounds, mosaic, bounds.Min, draw.Src)
		rgbaImg = tmp
	}

	// нанести символы
	err = DrawSymbolsOnImage(rgbaImg, matched, cellSize, "fonts/DejaVuSans.ttf")
	if err != nil {
		log.Printf("ошибка нанесения символов: %v", err)
	}

	return mosaic, usages, nil
}

// MatchToPalette строит матрицу подобранных цветов (gridW×gridH) на основе палитры.
func MatchToPalette(resized image.Image, palette []db.PaletteColor, gridW, gridH int) [][]db.PaletteColor {
	matched := make([][]db.PaletteColor, gridH)
	for y := 0; y < gridH; y++ {
		matched[y] = make([]db.PaletteColor, gridW)
		for x := 0; x < gridW; x++ {
			r, g, b, _ := resized.At(x, y).RGBA()
			pix := colorful.Color{
				R: float64(r) / 65535.0,
				G: float64(g) / 65535.0,
				B: float64(b) / 65535.0,
			}
			matched[y][x] = findNearestColor(pix, palette)
		}
	}
	return matched
}

// findNearestColor ищет ближайший цвет в палитре по евклидову дистанции в Lab.
func findNearestColor(c colorful.Color, palette []db.PaletteColor) db.PaletteColor {
	l1, a1, b1 := c.Lab()
	lab1 := [3]float64{l1, a1, b1}
	minDist := 1e9
	var nearest db.PaletteColor
	for _, pc := range palette {
		l2, a2, b2 := pc.Color.Lab()
		lab2 := [3]float64{l2, a2, b2}
		d := euclideanDistanceLab(lab1, lab2)
		if d < minDist {
			minDist = d
			nearest = pc
		}
	}
	return nearest
}

func euclideanDistanceLab(lab1, lab2 [3]float64) float64 {
	dL := lab1[0] - lab2[0]
	da := lab1[1] - lab2[1]
	db := lab1[2] - lab2[2]
	return dL*dL + da*da + db*db
}

// RenderMosaic строит итоговое изображение и подсчитывает количество элементов каждого цвета.
func RenderMosaic(matched [][]db.PaletteColor, cellSize int) (image.Image, []ColorUsage) {
	h := len(matched)
	w := len(matched[0])
	mosaic := image.NewRGBA(image.Rect(0, 0, w*cellSize, h*cellSize))
	usageMap := make(map[string]ColorUsage)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			pc := matched[y][x]
			u := usageMap[pc.DMCCode]
			if u.PaletteColor.DMCCode == "" {
				u.PaletteColor = pc
			}
			u.Count++
			usageMap[pc.DMCCode] = u

			nr, ng, nb := pc.Color.RGB255()
			rect := image.Rect(x*cellSize, y*cellSize, (x+1)*cellSize, (y+1)*cellSize)
			draw.Draw(mosaic, rect, &image.Uniform{C: color.RGBA{R: nr, G: ng, B: nb, A: 255}}, image.Point{}, draw.Src)
		}
	}
	// В срез
	usages := make([]ColorUsage, 0, len(usageMap))
	for _, u := range usageMap {
		usages = append(usages, u)
	}
	return mosaic, usages
}

// MedianFilter применяет медианный фильтр к изображению с ядром kernelSize (должно быть нечётным).
func MedianFilter(img image.Image, kernelSize int) image.Image {
	bounds := img.Bounds()
	filtered := image.NewRGBA(bounds)
	offset := kernelSize / 2

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			var rs, gs, bs []uint8
			for ky := -offset; ky <= offset; ky++ {
				for kx := -offset; kx <= offset; kx++ {
					nx := x + kx
					ny := y + ky
					if nx < bounds.Min.X || nx >= bounds.Max.X || ny < bounds.Min.Y || ny >= bounds.Max.Y {
						continue
					}
					r, g, b, _ := img.At(nx, ny).RGBA()
					rs = append(rs, uint8(r>>8))
					gs = append(gs, uint8(g>>8))
					bs = append(bs, uint8(b>>8))
				}
			}
			medR := median(rs)
			medG := median(gs)
			medB := median(bs)
			filtered.Set(x, y, color.RGBA{R: medR, G: medG, B: medB, A: 255})
		}
	}
	return filtered
}

// median вычисляет медиану из среза значений uint8.
func median(data []uint8) uint8 {
	n := len(data)
	// Простейшая сортировка
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			if data[j] < data[i] {
				data[i], data[j] = data[j], data[i]
			}
		}
	}
	return data[n/2]
}

// Нарисовать символы на мозаике
func DrawSymbolsOnImage(img *image.RGBA, matched [][]db.PaletteColor, cellSize int, fontPath string) error {
	fontBytes, err := ioutil.ReadFile(fontPath)
	if err != nil {
		return err
	}
	f, err := freetype.ParseFont(fontBytes)
	if err != nil {
		return err
	}
	c := freetype.NewContext()
	c.SetDPI(96)
	c.SetFont(f)
	c.SetFontSize(float64(cellSize) * 0.7) // размер символа чуть меньше клетки
	c.SetClip(img.Bounds())
	c.SetDst(img)
	c.SetSrc(image.Black) // цвет текста: чёрный

	for y := 0; y < len(matched); y++ {
		for x := 0; x < len(matched[0]); x++ {
			sym := matched[y][x].Symbol
			if sym == "" {
				continue // если не присвоено, пропускаем
			}
			// координаты центра клетки
			pt := freetype.Pt(
				x*cellSize+cellSize/4,   // x
				y*cellSize+cellSize*3/4, // y
			)
			_, err := c.DrawString(sym, pt)
			if err != nil {
				log.Printf("ошибка рисования символа %q: %v", sym, err)
			}
			// log.Printf("(%d,%d): %q", x, y, sym)

		}
	}
	return nil
}

func AssignSymbolsToMatched(matched [][]db.PaletteColor, allSymbols []string) {
	symbolMap := map[string]string{} // DMC -> символ
	symbolIdx := 0
	for y := 0; y < len(matched); y++ {
		for x := 0; x < len(matched[0]); x++ {
			pc := &matched[y][x]
			if pc.Symbol != "" {
				continue
			}
			if sym, ok := symbolMap[pc.DMCCode]; ok {
				pc.Symbol = sym
			} else {
				if symbolIdx < len(allSymbols) {
					pc.Symbol = allSymbols[symbolIdx]
					symbolMap[pc.DMCCode] = allSymbols[symbolIdx]
					symbolIdx++
				} else {
					pc.Symbol = "?" // если символы закончились
				}
			}
		}
	}
}
