package pdf

import (
    "bytes"
    "fmt"
    "image"
    "image/png"

    imagepkg "diamond-mosaic/internal/image"
    "github.com/jung-kurt/gofpdf"
)

// GeneratePDF генерирует PDF файл с мозаикой и легендой.
// Принимает на вход картинку-мозаику и список использованных цветов.
func GeneratePDF(mosaicImg image.Image, usages []imagepkg.ColorUsage) ([]byte, error) {
    var imgBuf bytes.Buffer
    if err := png.Encode(&imgBuf, mosaicImg); err != nil {
        return nil, fmt.Errorf("ошибка кодирования PNG: %v", err)
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

    // Вставка изображения (см. твой код)
    pdf.RegisterImageOptionsReader("mosaic", gofpdf.ImageOptions{ImageType: "PNG", ReadDpi: false}, &imgBuf)
    pageW, pageH := pdf.GetPageSize()
    info := pdf.GetImageInfo("mosaic")
    imgW, imgH := info.Width(), info.Height()
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
		//pdf.SetTextColor(0, 0, 0)     // чёрный символ
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
