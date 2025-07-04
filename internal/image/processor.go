package image

import (
	"diamond-mosaic/internal/db"
	"image"
	"image/color"
	"image/draw"
	"io"
	"io/ioutil"
	"log"
	"sync"
	"time"

	// "golang.org/x/image/font"

	"github.com/disintegration/imaging"
	"github.com/golang/freetype"
	"github.com/lucasb-eyer/go-colorful"
)

// allSymbols — набор символов для легенды схемы.
var allSymbols = []string{
	"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N", "O", "P", "Q", "R", "S", "T", "U", "V", "W", "X", "Y", "Z",
	"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o", "p", "q", "r", "s", "t", "u", "v", "w", "x", "y", "z",
	"0", "1", "2", "3", "4", "5", "6", "7", "8", "9",
	"!", "@", "#", "$", "%", "^", "&", "*", "(", ")", "-", "_", "+", "=", "~", "`",
	"[", "]", "{", "}", "<", ">", "?", "/", "\\", "|", ".", ",", ":", ";", "'", "\"",
}

// MosaicSizeInfo описывает физические и "штучные" размеры основы и вписанного изображения.
type MosaicSizeInfo struct {
	BaseWidthCM, BaseHeightCM int // Основа в см
	BaseWidthPX, BaseHeightPX int // Основа в "шт"
	ImgWidthCM, ImgHeightCM   int // Картинка в см (занятое пространство)
	ImgWidthPX, ImgHeightPX   int // Картинка в "шт"
}

// ColorUsage связывает цвет из палитры с количеством пикселей (алмазов).
type ColorUsage struct {
	PaletteColor db.PaletteColor
	Count        int
}

// Process декодирует входное изображение, превращает его в мозаичный рисунок
// и собирает список уникальных DMC-цветов с их количеством использования.
func Process(file io.Reader, palette []db.PaletteColor, widthCm int, heightCm int) (image.Image, []ColorUsage, MosaicSizeInfo, error) {
	// 1. Декодируем изображение
	src, err := imaging.Decode(file)
	if err != nil {
		return nil, nil, MosaicSizeInfo{}, err
	}

	// 2. Переводим см в мм и рассчитываем размеры сетки пользователя
	widthMm := float64(widthCm) * 10.0
	heightMm := float64(heightCm) * 10.0
	strazMm := 2.5
	userGridW := int(widthMm / strazMm)
	userGridH := int(heightMm / strazMm)
	srcW := src.Bounds().Dx()
	srcH := src.Bounds().Dy()

	// 3. Вписываем изображение в сетку основы (по центру)
	fitW, fitH, indexGrid := MakeFitIndexGrid(srcW, srcH, userGridW, userGridH)

	// 3. Масштабирование
	resized := imaging.Resize(src, fitW, fitH, imaging.CatmullRom)

	// 4. Фильтрация
	filtered := MedianFilter(resized, 3)

	// 5. Подбираем ближайшие цвета для каждого пикселя
	matched := MatchToPalette(filtered, palette, indexGrid)

	// 6. Назначаем символы цветам
	AssignSymbolsToMatched(matched, allSymbols)

	// 7. Генерируем картинку, считаем использование цветов
	const cellSize = 10
	_, usages := RenderMosaic(matched, cellSize)
	RemoveRareColors(matched, usages, 30) // удаляем редкие цвета
	mosaic, usages := RenderMosaic(matched, cellSize) 	// пересчитываем usages и картинку

	// 8. Конвертируем изображение в RGBA
	rgbaImg, ok := mosaic.(*image.RGBA)
	if !ok {
		bounds := mosaic.Bounds()
		tmp := image.NewRGBA(bounds)
		draw.Draw(tmp, bounds, mosaic, bounds.Min, draw.Src)
		rgbaImg = tmp
	}

	// 9. Наносим символы на изображение
	err = DrawSymbolsOnImage(rgbaImg, matched, cellSize, "fonts/DejaVuSans.ttf")
	if err != nil {
		log.Printf("ошибка нанесения символов: %v", err)
	}

	// 10. Формируем структуру с информацией о размерах
	sizeInfo := CalcMosaicSizeInfo(
		widthCm, heightCm, // пользовательские размеры
		userGridW, userGridH, // вся сетка основы
		fitW, fitH, // вписанное изображение
		2.5, // размер 1 алмаза в мм
	)

	return mosaic, usages, sizeInfo, nil
}

// CalcMosaicSizeInfo рассчитывает структуру с параметрами размеров мозаики и вписанного изображения.
func CalcMosaicSizeInfo(
	baseWcm, baseHcm int, // заданные пользователем см
	gridW, gridH int, // сетка основы (в алмазиках)
	imgFitW, imgFitH int, // размеры вписанного изображения в алмазиках
	cellSizeMM float64, // размер 1 алмаза в мм, например 2.5
) MosaicSizeInfo {

	// Переводим из алмазиков в сантиметры:
	imgWcm := int(float64(imgFitW) * cellSizeMM / 10.0)
	imgHcm := int(float64(imgFitH) * cellSizeMM / 10.0)

	return MosaicSizeInfo{
		BaseWidthCM:  baseWcm,
		BaseHeightCM: baseHcm,
		BaseWidthPX:  gridW,
		BaseHeightPX: gridH,
		ImgWidthCM:   imgWcm,
		ImgHeightCM:  imgHcm,
		ImgWidthPX:   imgFitW,
		ImgHeightPX:  imgFitH,
	}
}

// MatchToPalette подбирает к каждому пикселю (ячейке) ближайший цвет из палитры DMC.
func MatchToPalette(src image.Image, palette []db.PaletteColor, indexGrid [][][2]int) [][]db.PaletteColor {
	start := time.Now() // замер времени выполнения

	h := len(indexGrid)
	w := len(indexGrid[0])
	matched := make([][]db.PaletteColor, h)
	var wg sync.WaitGroup

	for y := 0; y < h; y++ {
		matched[y] = make([]db.PaletteColor, w)
		wg.Add(1)
		go func(y int) {
			defer wg.Done()
			for x := 0; x < w; x++ {
				idx := indexGrid[y][x]
				if idx[0] >= 0 && idx[1] >= 0 {
					r, g, b, _ := src.At(idx[0], idx[1]).RGBA()
					pix := colorful.Color{
						R: float64(r) / 65535.0,
						G: float64(g) / 65535.0,
						B: float64(b) / 65535.0,
					}
					matched[y][x] = findNearestColor(pix, palette)
				} else {
					matched[y][x] = db.PaletteColor{
						DMCCode: "BLANK", // Только для пустых областей!
						Name:    "Пусто",
						Color:   colorful.Color{R: 1, G: 1, B: 1},
						Symbol:  "",
					}
				}
			}
		}(y)
	}
	wg.Wait()
	elapsed := time.Since(start)
	log.Printf("[MatchToPalette] Время выполнения: %s", elapsed)
	return matched
}

// MakeFitIndexGrid рассчитывает размеры вписанной области (в клетках) с сохранением пропорций
// и возвращает размеры и индексы соответствия ячеек пикселям исходника.
func MakeFitIndexGrid(srcW, srcH, userGridW, userGridH int) (fitW, fitH int, pixelIndex [][][2]int) {
	fitW, fitH, offsetX, offsetY := ComputeFitArea(srcW, srcH, userGridW, userGridH)
	pixelIndex = make([][][2]int, userGridH)
	for y := 0; y < userGridH; y++ {
		pixelIndex[y] = make([][2]int, userGridW)
		for x := 0; x < userGridW; x++ {
			relX := x - offsetX
			relY := y - offsetY
			if relX >= 0 && relX < fitW && relY >= 0 && relY < fitH {
				pixelIndex[y][x] = [2]int{relX, relY}
			} else {
				pixelIndex[y][x] = [2]int{-1, -1}
			}
		}
	}
	return fitW, fitH, pixelIndex
}

// findNearestColor ищет ближайший цвет в палитре по евклидовой дистанции в Lab.
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

// euclideanDistanceLab вычисляет квадрат евклидова расстояния между двумя цветами в пространстве Lab.
func euclideanDistanceLab(lab1, lab2 [3]float64) float64 {
	dL := lab1[0] - lab2[0]
	da := lab1[1] - lab2[1]
	db := lab1[2] - lab2[2]
	return dL*dL + da*da + db*db
}

// RenderMosaic строит итоговое изображение и подсчитывает количество элементов каждого цвета.
func RenderMosaic(matched [][]db.PaletteColor, cellSize int) (image.Image, []ColorUsage) {
	start := time.Now()

	h := len(matched)
	w := len(matched[0])
	mosaic := image.NewRGBA(image.Rect(0, 0, w*cellSize, h*cellSize))
	usageMap := make(map[string]ColorUsage)

	var wg sync.WaitGroup

	for y := 0; y < h; y++ {
		wg.Add(1)
		go func(y int) {
			defer wg.Done()
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
				drawBorder(mosaic, rect, color.RGBA{R: 90, G: 90, B: 90, A: 255})
			}
		}(y)

		wg.Wait()
	}
	usages := make([]ColorUsage, 0, len(usageMap))
	for _, u := range usageMap {
		usages = append(usages, u)
	}
	elapsed := time.Since(start)
	log.Printf("[RenderMosaic] Время выполнения: %s", elapsed)

	return mosaic, usages
}

// RemoveRareColors заменяет редкие цвета на ближайшие частые.
func RemoveRareColors(matched [][]db.PaletteColor, usages []ColorUsage, minCount int) {
	// 1. Собираем частые и редкие цвета
	majorColors := map[string]db.PaletteColor{} // частые цвета
	minorColors := map[string]db.PaletteColor{} // редкие цвета
	for _, u := range usages {
		if u.PaletteColor.DMCCode == "BLANK" {
			continue // игнорируем BLANK
		}
		if u.Count >= minCount {
			majorColors[u.PaletteColor.DMCCode] = u.PaletteColor
		} else {
			minorColors[u.PaletteColor.DMCCode] = u.PaletteColor
		}
	}
	if len(majorColors) == 0 {

		return
	}

	// 2. Для каждого редкого цвета ищем ближайший частый
	findNearestMajor := func(c colorful.Color) db.PaletteColor {
		l1, a1, b1 := c.Lab()
		minDist := 1e9
		var nearest db.PaletteColor
		for _, pc := range majorColors {
			l2, a2, b2 := pc.Color.Lab()
			dl := l1 - l2
			da := a1 - a2
			db_ := b1 - b2
			dist := dl*dl + da*da + db_*db_
			if dist < minDist {
				minDist = dist
				nearest = pc
			}
		}
		return nearest
	}

	// Заменить все пиксели с "редкими" цветами на ближайший частый
	for y := 0; y < len(matched); y++ {
		for x := 0; x < len(matched[0]); x++ {
			pc := matched[y][x]
			if pc.DMCCode == "BLANK" {
				continue // не трогаем фон
			}
			if _, isMinor := minorColors[pc.DMCCode]; isMinor {
				matched[y][x] = findNearestMajor(pc.Color)
			}
		}
	}
}

// MedianFilter применяет медианный фильтр к изображению с ядром kernelSize.
func MedianFilter(img image.Image, kernelSize int) image.Image {
	start := time.Now() // замер времени выполнения
	var wg sync.WaitGroup

	bounds := img.Bounds()
	filtered := image.NewRGBA(bounds)
	offset := kernelSize / 2

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		wg.Add(1)
		go func(y int) {
			defer wg.Done()
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
		}(y)
	}

	wg.Wait()
	elapsed := time.Since(start)
	log.Printf("[MedianFilter] Время выполнения: %s", elapsed)

	return filtered
}

// median возвращает медиану из среза uint8.
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

// DrawSymbolsOnImage наносит символы на итоговое изображение-мозаику.
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

	const brightnessThreshold = 0.5 // порог

	for y := 0; y < len(matched); y++ {
		for x := 0; x < len(matched[0]); x++ {
			pc := matched[y][x]
			if pc.Symbol == "" || pc.DMCCode == "BLANK" {
				continue // если не присвоено, пропускаем
			}

			c.SetSrc(chooseSymbolColor(pc.Color, brightnessThreshold))
			// координаты центра клетки
			pt := freetype.Pt(
				x*cellSize+cellSize/5,   // x
				y*cellSize+cellSize*5/6, // y
			)
			_, err := c.DrawString(pc.Symbol, pt)
			if err != nil {
				log.Printf("ошибка рисования символа %q: %v", pc.Symbol, err)
			}
		}
	}

	return nil
}

// chooseSymbolColor возвращает чёрный или белый цвет для символа по яркости фона.
func chooseSymbolColor(col colorful.Color, threshold float64) image.Image {
	l, _, _ := col.Lab()
	if l > threshold {
		return image.Black // ← image.Black, а не color.Black
	}
	return image.White
}

// AssignSymbolsToMatched назначает каждому цвету уникальный символ.
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

// drawBorder рисует рамку вокруг одной клетки мозаики.
func drawBorder(img *image.RGBA, rect image.Rectangle, c color.Color) {
	minX, minY, maxX, maxY := rect.Min.X, rect.Min.Y, rect.Max.X, rect.Max.Y

	// Горизонтальные линии
	for x := minX; x < maxX; x++ {
		img.Set(x, minY, c)   // верхняя
		img.Set(x, maxY-1, c) // нижняя
	}
	// Вертикальные линии
	for y := minY; y < maxY; y++ {
		img.Set(minX, y, c)   // левая
		img.Set(maxX-1, y, c) // правая
	}
}

// ComputeFitArea вычисляет размеры вписанной области (в клетках) с сохранением пропорций
func ComputeFitArea(srcW, srcH, userGridW, userGridH int) (fitW, fitH, offsetX, offsetY int) {
	srcRatio := float64(srcW) / float64(srcH)
	userRatio := float64(userGridW) / float64(userGridH)

	if srcRatio > userRatio {
		fitW = userGridW
		fitH = int(float64(userGridW) / srcRatio)
		offsetX = 0
		offsetY = 0 // ← прижимаем к верху!
	} else {
		fitH = userGridH
		fitW = int(float64(userGridH) * srcRatio)
		offsetY = 0
		offsetX = 0 // ← прижимаем к левому краю!
	}
	return
}
