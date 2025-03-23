document.getElementById("uploadForm").addEventListener("submit", async function (e) {
    e.preventDefault();
  
    const formData = new FormData(this);
    const status = document.getElementById("status");
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
      a.download = "mosaic.png";
      a.click();
      window.URL.revokeObjectURL(url);
  
      status.textContent = "Готово! PNG-файл скачан.";
    } catch (err) {
      console.error(err);
      status.textContent = "Произошла ошибка при генерации схемы.";
    }
  });
  