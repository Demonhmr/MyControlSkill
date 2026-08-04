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
//
// Лаунчер работает без бэкенда: в этом режиме приложение хранит данные
// в localStorage браузера. Серверный режим — см. cmd/server.
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
	"runtime"
	"syscall"
	"time"

	"mycontrolskill/internal/web"
)

//go:embed all:dist
var embedded embed.FS

const appName = "Компас руководителя"

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
		Handler:           web.SPA(files),
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
