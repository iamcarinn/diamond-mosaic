package handlers

import (
	"bytes"
	"fmt"
	"image/png"
	"log"
	"net/http"
	"strconv"

	"diamond-mosaic/internal/db"
	"diamond-mosaic/internal/image"

	"github.com/jung-kurt/gofpdf"
)

// Palette — глобальная палитра из БД.
var Palette []db.PaletteColor

func SetPaletteFromDB(p []db.PaletteColor) {
	Palette = p
}

// GenerateHandler отдаёт PDF с мозаикой (сверху) и легендой, разбитой на страницы по необходимости.
func GenerateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

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

	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Ошибка получения файла", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Получаем мозаичное изображение и статистику использования цветов
	mosaicImg, usages, err := image.Process(file, Palette, widthCm, heightCm)
	if err != nil {
		log.Printf("Ошибка обработки изображения: %v", err)
		http.Error(w, fmt.Sprintf("Ошибка обработки изображения: %v", err), http.StatusInternalServerError)
		return
	}

	// // Символы для легенды
	// var allSymbols = []string{
	// 	"A","B","C","D","E","F","G","H","I","J","K","L","M","N","O","P","Q","R","S","T","U","V","W","X","Y","Z",
	// 	"a","b","c","d","e","f","g","h","i","j","k","l","m","n","o","p","q","r","s","t","u","v","w","x","y","z",
	// 	"0","1","2","3","4","5","6","7","8","9",
	// 	"!","@","#","$","%","^","&","*","(",")","-","_","+","=","~","`",
	// 	"[","]","{","}","<",">","?","/","\\","|",".",",",":",";","'","\"",
    // 	// "Ж","З","И","Й","Л","П","Ф","Ц","Ч","Ш","Щ","Э","Ю","Я",
    // 	// "ж","з","и","й","л","п","ф","ц","ч","ш","щ","э","ю","я",
	// }

	// // Присваиваем символы только используемым цветам
	// for i := 0; i < len(usages) && i < len(allSymbols); i++ {
	// 	usages[i].PaletteColor.Symbol = allSymbols[i]
	// }

	// // Логируем все используемые цвета, их DMC-код, название и присвоенный символ
	// for i, u := range usages {
	// 	log.Printf("Цвет #%d: DMC=%s, Name=%s, Symbol=%s, Count=%d",
	// 		i+1, u.PaletteColor.DMCCode, u.PaletteColor.Name, u.PaletteColor.Symbol, u.Count)
	// }

	// Кодируем PNG в буфер
	var imgBuf bytes.Buffer
	if err := png.Encode(&imgBuf, mosaicImg); err != nil {
		http.Error(w, "Ошибка кодирования PNG", http.StatusInternalServerError)
		return
	}

	// Константы расположения
	const (
		pageMarginTop   = 15.0 // мм от верха для первой страницы
		legendMarginTop = 8.0  // между картинкой и легендой
		bottomMargin    = 15.0 // мм от низа страницы
		cols            = 7
		squareSize      = 5.0
		gutter          = 2.0
		marginLeft      = 10.0
		marginRight     = 10.0
	)

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()

	// Вставляем мозаичную картинку
	pdf.RegisterImageOptionsReader("mosaic", gofpdf.ImageOptions{ImageType: "PNG", ReadDpi: false}, &imgBuf)
	pageW, pageH := pdf.GetPageSize()
	info := pdf.GetImageInfo("mosaic")
	imgW, imgH := info.Width(), info.Height()
	// масштабируем до 90% ширины
	maxW := pageW - marginLeft - marginRight
	scale := maxW / imgW
	if imgH*scale > (pageH - pageMarginTop - legendMarginTop - bottomMargin) {
		scale = (pageH - pageMarginTop - legendMarginTop - bottomMargin) / imgH
	}
	imgW, imgH = imgW*scale, imgH*scale
	x0 := (pageW - imgW) / 2
	y0 := pageMarginTop
	pdf.ImageOptions("mosaic", x0, y0, imgW, imgH, false, gofpdf.ImageOptions{ImageType: "PNG", ReadDpi: false}, 0, "")

	// ГОТОВИМ ПЕРЕМЕННЫЕ ДЛЯ ЛЕГЕНДЫ
	pdf.SetFont("Arial", "", 8)
	usableW := pageW - marginLeft - marginRight
	colW := usableW / float64(cols)
	lineH := squareSize + 1.0

	currentCol := 0
	currentRow := 0
	startY := y0 + imgH + legendMarginTop

	// Функция начала новой страницы для легенды
	newPage := func() {
		pdf.AddPage()
		pdf.SetFont("Arial", "", 8)
		currentCol = 0
		currentRow = 0
		startY = pageMarginTop
	}

	// Рисуем каждую запись легенды
	for _, u := range usages {
		// позиция этого квадрата
		xPos := marginLeft + float64(currentCol)*colW
		yPos := startY + float64(currentRow)*lineH

		// если не помещается вниз — новая страница
		if yPos+squareSize > pageH-bottomMargin {
			newPage()
			xPos = marginLeft
			yPos = startY
		}

		// квадрат
		rC, gC, bC := u.PaletteColor.Color.RGB255()
		pdf.SetFillColor(int(rC), int(gC), int(bC))
		pdf.Rect(xPos, yPos, squareSize, squareSize, "F")

		// === Символ по центру квадратика ===
		pdf.SetFont("Arial", "B", 10) // жирный шрифт, размер 10
		pdf.SetTextColor(0, 0, 0)     // чёрный символ
		symbol := u.PaletteColor.Symbol
		fontSize := 10.0 // размер шрифта, должен совпадать с SetFont
		// Получаем ширину символа (в миллиметрах)
		symbolWidth := pdf.GetStringWidth(symbol)
		// X — по центру квадрата
		symbolX := xPos + (squareSize-symbolWidth)/2
		// Y — по центру квадрата (подбор вручную для красоты)
		symbolY := yPos + (squareSize+fontSize)/2 - 3.5
		pdf.Text(symbolX, symbolY, symbol)
		// Вернуть обычный шрифт для текста справа
		pdf.SetFont("Arial", "", 8)

		// текст справа: DMC + (количество)
		text := fmt.Sprintf("%s (%d)", u.PaletteColor.DMCCode, u.Count)
		textX := xPos + squareSize + gutter
		textY := yPos + squareSize - 1.0
		pdf.Text(textX, textY, text)

		// следующие столбец/строка
		currentCol++
		if currentCol >= cols {
			currentCol = 0
			currentRow++
		}
	}

	// Генерируем PDF и отдаём клиенту
	var pdfBuf bytes.Buffer
	if err := pdf.Output(&pdfBuf); err != nil {
		http.Error(w, "Ошибка формирования PDF", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="mosaic_with_legend.pdf"`)
	if _, err := w.Write(pdfBuf.Bytes()); err != nil {
		log.Printf("Ошибка записи ответа: %v", err)
	}
}
