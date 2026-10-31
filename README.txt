recast — конвертер видео, аудио и картинок на ffmpeg
=====================================================

Портативная версия: ничего не устанавливает. Распакуйте папку куда угодно
и запустите recast.exe. Настройки сохраняются в папку data рядом с exe
(если папка только для чтения — в %APPDATA%\recast).

Как пользоваться
  1. Перетащите файлы или папки в окно (или «+ файлы», Ctrl+O).
  2. Справа выберите формат: видео, аудио или картинка.
  3. Нажмите «конвертировать». Готовые файлы появятся рядом с исходниками
     или в выбранной папке; существующие файлы не перезаписываются.

Форматы
  Видео:    MP4, MKV, MOV, WEBM, AVI, M4V, WMV, FLV, TS, MPG, OGV, GIF
  Аудио:    MP3, M4A, AAC, WAV, FLAC, OGG, OPUS, WMA, AIFF, AC3, ALAC
  Картинки: PNG, JPG, WEBP, AVIF, BMP, TIFF, ICO
  Любой другой — в «Свои параметры ffmpeg» (расширение и/или аргументы).
  Кодирование на видеокарте: NVIDIA NVENC, Intel QSV, AMD AMF — если есть.

Две сборки
  recast-…-win64.zip       с ffmpeg внутри, работает сразу.
  recast-…-win64-lite.zip  без ffmpeg: положите ffmpeg.exe и ffprobe.exe
                           рядом с recast.exe, установите ffmpeg в PATH
                           или укажите путь в «Настройках».

Требования: Windows 10/11 x64 и WebView2 Runtime (в Windows 11 уже есть).

ffmpeg
  В полную сборку входит ffmpeg 9.0.2 (full build, www.gyan.dev) под лицензией
  GPL v3 — см. ffmpeg-LICENSE.txt. Исходный код ffmpeg: https://ffmpeg.org,
  точный коммит сборки указан в ffmpeg-README.txt.


recast — video, audio and image converter on ffmpeg
===================================================

Portable: nothing is installed. Unpack the folder anywhere and run
recast.exe. Settings live in the data folder next to the exe
(%APPDATA%\recast if that folder is read-only).

How to use
  1. Drop files or folders onto the window (or "+ files", Ctrl+O).
  2. Pick a format on the right: video, audio or image.
  3. Press "convert". Results go next to the sources or into the chosen
     folder; existing files are never overwritten.

Formats
  Video:  MP4, MKV, MOV, WEBM, AVI, M4V, WMV, FLV, TS, MPG, OGV, GIF
  Audio:  MP3, M4A, AAC, WAV, FLAC, OGG, OPUS, WMA, AIFF, AC3, ALAC
  Images: PNG, JPG, WEBP, AVIF, BMP, TIFF, ICO
  Anything else: "Own ffmpeg parameters" (extension and/or arguments).
  GPU encoding: NVIDIA NVENC, Intel QSV, AMD AMF when available.

Two builds
  recast-…-win64.zip       ffmpeg included, works right away.
  recast-…-win64-lite.zip  no ffmpeg: put ffmpeg.exe and ffprobe.exe next to
                           recast.exe, have ffmpeg on PATH, or set the path
                           in Settings.

Requires Windows 10/11 x64 and the WebView2 Runtime (built into Windows 11).

ffmpeg
  The full build ships ffmpeg 9.0.2 (full build, www.gyan.dev) under GPL v3,
  see ffmpeg-LICENSE.txt. ffmpeg source: https://ffmpeg.org; the exact commit
  of this build is in ffmpeg-README.txt.
