// Пакет web отдаёт собранную статику Vite.
//
// Используется и лаунчером (статика вшита в бинарник через go:embed), и
// сервером (статика читается с диска), поэтому принимает произвольную fs.FS.
package web

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

const indexFile = "index.html"

// spaHandler отдаёт статику, а на неизвестные пути возвращает index.html,
// чтобы клиентская навигация не упиралась в 404.
type spaHandler struct {
	files fs.FS
}

// SPA возвращает обработчик статики одностраничного приложения.
func SPA(files fs.FS) http.Handler {
	return spaHandler{files: files}
}

// openFile открывает обычный файл по имени. Каталоги считаются отсутствующими:
// отдавать листинг незачем, а клиентский роут всё равно уйдёт в оболочку.
func (h spaHandler) openFile(name string) (fs.File, fs.FileInfo, bool) {
	f, err := h.files.Open(name)
	if err != nil {
		return nil, nil, false
	}
	info, err := f.Stat()
	if err != nil || info.IsDir() {
		f.Close()
		return nil, nil, false
	}
	return f, info, true
}

func (h spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if name == "" || name == "." {
		name = indexFile
	}

	f, info, ok := h.openFile(name)
	if !ok {
		// Неизвестный путь — отдаём оболочку приложения.
		name = indexFile
		if f, info, ok = h.openFile(name); !ok {
			http.Error(w, "index.html не найден в сборке", http.StatusInternalServerError)
			return
		}
	}
	defer f.Close()

	seeker, ok := f.(interface {
		Read([]byte) (int, error)
		Seek(int64, int) (int64, error)
	})
	if !ok {
		http.Error(w, "файл не поддерживает чтение со смещением", http.StatusInternalServerError)
		return
	}

	// Ассеты Vite содержат хэш в имени, поэтому кэшируются безопасно.
	if strings.HasPrefix(name, "assets/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}
	http.ServeContent(w, r, name, info.ModTime(), seeker)
}
