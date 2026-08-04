// Сервер «Компаса руководителя».
//
// В отличие от cmd/launcher (демо на localStorage, один .exe без бэкенда)
// это сетевой режим: данные лежат в SQLite, респонденты заполняют анкеты
// 360° по своим ссылкам со своих устройств.
//
// Настройки — из окружения, см. internal/config.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mycontrolskill/internal/config"
	"mycontrolskill/internal/store"
	"mycontrolskill/internal/web"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	if err := run(log); err != nil {
		log.Error("сервер остановлен с ошибкой", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// Контекст живёт до первого SIGINT/SIGTERM и гасит долгие операции
	// запуска (например, миграции) при остановке.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	st, err := store.Open(ctx, cfg.DBPath)
	if err != nil {
		return err
	}
	defer st.Close()
	log.Info("база готова", "path", cfg.DBPath)

	if _, err := os.Stat(cfg.StaticDir); err != nil {
		// Не фатально: API работает и без фронта, но знать об этом надо.
		log.Warn("каталог со сборкой фронтенда недоступен, отдаваться будет только API",
			"dir", cfg.StaticDir, "err", err)
	}

	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           routes(st, cfg.StaticDir),
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("слушаю", "addr", cfg.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Info("получен сигнал, завершаюсь")
	}

	// Контекст завершения отдельный: исходный уже отменён сигналом.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	return server.Shutdown(shutdownCtx)
}

func routes(st *store.Store, staticDir string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthz(st))
	// Всё остальное — оболочка SPA. Обработчики /api/... появятся следующими
	// шагами и перехватят свои пути раньше этого маршрута.
	mux.Handle("/", web.SPA(os.DirFS(staticDir)))
	return mux
}

// healthz отвечает 200 только если база действительно отвечает: иначе
// балансировщик будет слать трафик на экземпляр с отвалившимся SQLite.
func healthz(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		status := http.StatusOK
		body := map[string]string{"status": "ok"}
		if err := st.Ping(ctx); err != nil {
			status = http.StatusServiceUnavailable
			body = map[string]string{"status": "db unavailable"}
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(body)
	}
}
