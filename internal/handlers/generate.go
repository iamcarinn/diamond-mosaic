package handlers

import (
	"bytes"
	"fmt"
	"image/png"
	"log"
	"net/http"

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
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Ошибка получения файла", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Получаем мозаичное изображение и статистику использования цветов
	mosaicImg, usages, err := image.Process(file, Palette)
	if err != nil {
		log.Printf("Ошибка обработки изображения: %v", err)
		http.Error(w, fmt.Sprintf("Ошибка обработки изображения: %v", err), http.StatusInternalServerError)
		return
	}

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
		cols            = 8
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

		// текст: код + (кол-во)
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
