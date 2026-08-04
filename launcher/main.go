// Лаунчер прототипа «Компас руководителя».
//
// Собранная статика Vite вшивается в бинарник (go:embed), поэтому .exe
// самодостаточен: ни Node.js, ни распакованных файлов рядом не нужно.
// При запуске поднимается локальный HTTP-сервер на свободном порту и
// открывается браузер по умолчанию.
//
// Через file:// прототип открыть нельзя: сборка Vite использует ES-модули,
// а браузеры блокируют их загрузку с локальной файловой системы (CORS).
// Отсюда и локальный сервер вместо простого открытия index.html.
package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"runtime"
	"strings"
	"syscall"
	"time"
)

//go:embed all:dist
var embedded embed.FS

const appName = "Компас руководителя"

// spaHandler отдаёт статику, а на неизвестные пути возвращает index.html,
// чтобы клиентская навигация не упиралась в 404.
type spaHandler struct {
	files fs.FS
}

func (h spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if name == "" || name == "." {
		name = "index.html"
	}
	f, err := h.files.Open(name)
	if err != nil {
		// Неизвестный путь — отдаём оболочку приложения.
		name = "index.html"
		f, err = h.files.Open(name)
		if err != nil {
			http.Error(w, "index.html не найден в сборке", http.StatusInternalServerError)
			return
		}
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil || info.IsDir() {
		http.Error(w, "не файл", http.StatusNotFound)
		return
	}
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

// openBrowser открывает URL в браузере по умолчанию.
func openBrowser(url string) error {
	switch runtime.GOOS {
	case "windows":
		// rundll32 надёжнее `cmd /c start`: не ломается на символах & в URL
		// и не требует экранирования.
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		return exec.Command("open", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}

func main() {
	files, err := fs.Sub(embedded, "dist")
	if err != nil {
		fatal("не удалось прочитать вшитую сборку: %v", err)
	}

	// Порт 0 — операционная система сама выдаст свободный, так что второй
	// запущенный экземпляр не конфликтует с первым.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fatal("не удалось занять локальный порт: %v", err)
	}
	url := fmt.Sprintf("http://%s/", listener.Addr().String())

	server := &http.Server{
		Handler:           spaHandler{files: files},
		ReadHeaderTimeout: 10 * time.Second,
	}

	fmt.Printf("\n  %s — прототип\n\n", appName)
	fmt.Printf("  Адрес:  %s\n", url)
	fmt.Printf("  Данные сохраняются в браузере (localStorage), никуда не отправляются.\n\n")
	fmt.Printf("  Не закрывайте это окно, пока работаете с приложением.\n")
	fmt.Printf("  Для выхода: Ctrl+C или просто закройте окно.\n\n")

	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fatal("сервер остановился: %v", err)
		}
	}()

	if err := openBrowser(url); err != nil {
		fmt.Printf("  Не удалось открыть браузер автоматически (%v).\n", err)
		fmt.Printf("  Откройте адрес выше вручную.\n\n")
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	fmt.Printf("\n  Завершение…\n")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "\nОшибка: "+format+"\n", args...)
	fmt.Fprintf(os.Stderr, "Нажмите Enter, чтобы закрыть окно…\n")
	fmt.Fscanln(os.Stdin)
	os.Exit(1)
}
