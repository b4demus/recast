# recast

Портативный конвертер видео, аудио и изображений для Windows на базе FFmpeg. Написан на Go и Wails; интерфейс — HTML, CSS и JavaScript без отдельной сборки фронтенда.

[Скачать последнюю версию](https://github.com/b4demus/recast/releases/latest) · [English](#english)

## Возможности

- Перетаскивание файлов и папок, очередь и параллельная конвертация.
- Видео: MP4, MKV, MOV, WEBM, AVI, M4V, WMV, FLV, TS, MPG, OGV, GIF.
- Аудио: MP3, M4A, AAC, WAV, FLAC, OGG, OPUS, WMA, AIFF, AC3, ALAC.
- Изображения: PNG, JPG, WEBP, AVIF, BMP, TIFF, ICO.
- Аппаратное кодирование NVIDIA NVENC, Intel QSV и AMD AMF при наличии поддержки.
- Выбор качества и разрешения, собственные параметры FFmpeg.
- Русский и английский интерфейс, светлая и тёмная темы.

Доступность форматов и кодировщиков зависит от сборки FFmpeg и оборудования.

## Установка и запуск

Требуются Windows 10/11 x64 и WebView2 Runtime.

1. Скачайте ZIP из [Releases](https://github.com/b4demus/recast/releases).
2. Распакуйте архив и запустите `recast.exe`.
3. Перетащите файлы в окно, выберите формат и нажмите «конвертировать».

`recast-…-win64.zip` включает FFmpeg и FFprobe. Вариант `recast-…-win64-lite.zip` поставляется без них: положите `ffmpeg.exe` и `ffprobe.exe` рядом с приложением, добавьте их в `PATH` или укажите путь в настройках.

Результаты сохраняются рядом с исходниками или в выбранную папку. По умолчанию существующие файлы не перезаписываются. Настройки хранятся в папке `data` рядом с приложением; если она недоступна для записи — в `%APPDATA%\recast`.

## Сборка из исходников

В Windows установите Go 1.27+, Wails CLI v2.17.0 и WebView2 Runtime. Для тестов и упаковки полной версии нужны FFmpeg и FFprobe в `PATH`.

```powershell
git clone https://github.com/b4demus/recast.git
cd recast
go install github.com/wailsapp/wails/v2/cmd/wails@v2.17.0
.\build.ps1
```

Готовое приложение: `dist\recast\recast.exe`. Если команда `wails` не найдена, добавьте `%USERPROFILE%\go\bin` в `PATH`.

```powershell
go test -count=1 ./...       # тесты, включая конвертацию доступными кодировщиками
.\build.ps1 -FFmpeg         # скопировать FFmpeg и FFprobe рядом с приложением
.\build.ps1 -Release        # создать полную и lite ZIP-сборки в release
```

Файлы интерфейса находятся в `frontend/dist` и встраиваются в приложение через `go:embed`. Node.js и npm для сборки не требуются. Для просмотра интерфейса в браузере можно открыть `frontend/dist/index.html`; в этом режиме используются демонстрационные данные.

## Лицензия

Исходный код recast распространяется под [GNU GPL v3](LICENSE). Полная ZIP-сборка включает FFmpeg от Gyan.dev под GPL v3; лицензия и сведения о точной версии находятся в `ffmpeg-LICENSE.txt` и `ffmpeg-README.txt` внутри архива. JetBrains Mono распространяется под [SIL Open Font License](frontend/dist/fonts/OFL.txt). Зависимости сохраняют собственные лицензии.

## English

Portable video, audio and image converter for Windows, powered by FFmpeg. Built with Go and Wails, with a plain HTML/CSS/JavaScript interface.

- Drag and drop files or folders, queue conversions and run jobs in parallel.
- Convert video, audio and images; choose quality, resolution or custom FFmpeg arguments.
- Use NVIDIA NVENC, Intel QSV or AMD AMF when supported by your hardware and FFmpeg build.
- Russian and English interface, light and dark themes.

Requires Windows 10/11 x64 and WebView2 Runtime. Download a ZIP from [Releases](https://github.com/b4demus/recast/releases), extract it and run `recast.exe`. The full build includes FFmpeg and FFprobe; the lite build requires them beside the executable, on `PATH`, or configured in Settings.

To build on Windows, install Go 1.27+, Wails CLI v2.17.0 and WebView2 Runtime, then run `.\build.ps1`. FFmpeg and FFprobe on `PATH` enable conversion tests and release packaging. The frontend in `frontend/dist` is embedded directly; Node.js is not required.

Licensed under [GNU GPL v3](LICENSE). Bundled FFmpeg and the JetBrains Mono font retain their respective licenses, as described above.
