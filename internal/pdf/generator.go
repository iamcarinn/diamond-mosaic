package pdf

import (
	"bytes"
	"fmt"
	"image"
	"image/png"

	imagepkg "diamond-mosaic/internal/image"

	"github.com/jung-kurt/gofpdf"
)

// GeneratePDF формирует PDF-файл с мозаикой и легендой.
func GeneratePDF(mosaicImg image.Image, usages []imagepkg.ColorUsage, sizeInfo imagepkg.MosaicSizeInfo) ([]byte, error) {
	// 1. Кодируем картинку-мозаику в PNG-буфер
	var imgBuf bytes.Buffer
	if err := png.Encode(&imgBuf, mosaicImg); err != nil {
		return nil, fmt.Errorf("ошибка кодирования PNG: %v", err)
	}

	// 2. Описываем параметры разметки PDF (можно вынести в структуру PDFLayout)
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
	pageW, pageH := pdf.GetPageSize()

	// 3. Выводим размеры основы и изображения над схемой
	y0 := printMosaicSizes(pdf, sizeInfo, pageW, pageMarginTop)

	// 4. Регистрируем изображение и вставляем его в PDF
	pdf.RegisterImageOptionsReader("mosaic", gofpdf.ImageOptions{ImageType: "PNG", ReadDpi: false}, &imgBuf)
	info := pdf.GetImageInfo("mosaic")
	imgW, imgH := info.Width(), info.Height()
	maxW := pageW - marginLeft - marginRight
	scale := maxW / imgW
	if imgH*scale > (pageH - pageMarginTop - legendMarginTop - bottomMargin) {
		scale = (pageH - pageMarginTop - legendMarginTop - bottomMargin) / imgH
	}
	imgW, imgH = imgW*scale, imgH*scale
	x0 := (pageW - imgW) / 2
	pdf.ImageOptions("mosaic", x0, y0, imgW, imgH, false, gofpdf.ImageOptions{ImageType: "PNG", ReadDpi: false}, 0, "")

	// 5. Готовим переменные для легенды (таблицы цветов)
	pdf.SetFont("Arial", "", 8)
	usableW := pageW - marginLeft - marginRight
	colW := usableW / float64(cols)
	lineH := squareSize + 1.0

	currentCol := 0
	currentRow := 0
	startY := y0 + imgH + legendMarginTop

	// 6. Вспомогательная функция для начала новой страницы легенды
	newPage := func() {
		pdf.AddPage()
		pdf.SetFont("Arial", "", 8)
		currentCol = 0
		currentRow = 0
		startY = pageMarginTop
	}

	// 7. Рисуем каждый элемент легенды (цвет, символ, количество)
	for _, u := range usages {
		// не выводим пустые
		if u.PaletteColor.DMCCode == "BLANK" {
			continue
		}
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
		// Определяем яркость цвета фона
		l, _, _ := u.PaletteColor.Color.Lab()
		if l > 0.5 {
			pdf.SetTextColor(0, 0, 0) // светлый фон → чёрный текст
		} else {
			pdf.SetTextColor(255, 255, 255) // тёмный фон → белый текст
		}

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
		pdf.SetTextColor(0, 0, 0)
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

	// Возврат готового PDF как []byte
	var pdfBuf bytes.Buffer
	if err := pdf.Output(&pdfBuf); err != nil {
		return nil, fmt.Errorf("ошибка формирования PDF: %v", err)
	}
	return pdfBuf.Bytes(), nil
}

// printMosaicSizes выводит текст с размерами над изображением
func printMosaicSizes(pdf *gofpdf.Fpdf, size imagepkg.MosaicSizeInfo, pageW float64, pageMarginTop float64) float64 {
	pdf.AddUTF8Font("DejaVu", "", "fonts/DejaVuSans.ttf")
	pdf.SetFont("DejaVu", "", 12)
	pdf.SetTextColor(60, 70, 160)
	baseStr := fmt.Sprintf(
		"Размер основы: %d x %d см (%d x %d шт)",
		size.BaseWidthCM, size.BaseHeightCM, size.BaseWidthPX, size.BaseHeightPX,
	)
	imgStr := fmt.Sprintf(
		"Размер изображения: %d x %d см (%d x %d шт)",
		size.ImgWidthCM, size.ImgHeightCM, size.ImgWidthPX, size.ImgHeightPX,
	)

	// Центрируем по ширине
	pdf.SetXY(0, pageMarginTop-10)
	pdf.CellFormat(pageW, 7, baseStr, "", 1, "C", false, 0, "")
	pdf.SetX(0)
	pdf.CellFormat(pageW, 7, imgStr, "", 1, "C", false, 0, "")

	// Вернём новую позицию по Y (для картинки)
	return pdf.GetY() + 3 // +3 мм — небольшой отступ после текста
}
