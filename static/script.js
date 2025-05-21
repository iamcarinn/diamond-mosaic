document.getElementById("uploadForm").addEventListener("submit", async function (e) {
  e.preventDefault();

  const input = this.querySelector('input[name="file"]');
  const status = document.getElementById("status");
  status.textContent = "";

  // Проверяем, выбран ли файл
  if (!input.files || input.files.length === 0) {
    status.textContent = "Файл не выбран.";
    return;
  }

  const file = input.files[0];
  // Проверяем MIME-тип
  if (file.type !== "image/png") {
    status.textContent = "Пожалуйста, загрузите файл в формате PNG.";
    return;
  }
  
  // Всё ок, начинаем отправку
  const formData = new FormData(this);
  status.textContent = "Обработка изображения...";

  try {
    const response = await fetch("/generate", {
      method: "POST",
      body: formData,
    });

    if (!response.ok) throw new Error("Ошибка при генерации схемы");

    const blob = await response.blob();
    const url = window.URL.createObjectURL(blob);

    const a = document.createElement("a");
    a.href = url;
    a.download = "mosaic.pdf";
    a.click();
    window.URL.revokeObjectURL(url);

    status.textContent = "Готово! PDF-файл скачан.";
  } catch (err) {
    console.error(err);
    status.textContent = "Произошла ошибка при генерации схемы.";
  }
});
