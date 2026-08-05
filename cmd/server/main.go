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
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mycontrolskill/internal/api"
	"mycontrolskill/internal/config"
	"mycontrolskill/internal/mail"
	"mycontrolskill/internal/store"
	"mycontrolskill/internal/web"
)

func main() {
	// Копия делается тем же бинарником: утилиты sqlite3 на сервере может не
	// быть, а копировать файл базы при включённом WAL небезопасно.
	backupPath := flag.String("backup", "", "сделать копию базы в указанный файл и выйти")
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	var err error
	if *backupPath != "" {
		err = backup(log, *backupPath)
	} else {
		err = run(log)
	}
	if err != nil {
		log.Error("завершение с ошибкой", "err", err)
		os.Exit(1)
	}
}

// backup делает копию базы и выходит. Служба при этом может работать:
// SQLite в режиме WAL допускает чтение из другого процесса.
func backup(log *slog.Logger, destPath string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx := context.Background()
	st, err := store.Open(ctx, cfg.DBPath)
	if err != nil {
		return err
	}
	defer st.Close()

	if err := st.Backup(ctx, destPath); err != nil {
		return err
	}
	log.Info("копия базы создана", "from", cfg.DBPath, "to", destPath)
	return nil
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

	mailer, err := newMailer(cfg, log)
	if err != nil {
		return err
	}

	apiServer := &api.Server{
		Store:         st,
		Mailer:        mailer,
		Log:           log,
		BaseURL:       cfg.BaseURL,
		SecureCookies: cfg.SecureCookies(),
		TrustProxy:    cfg.TrustProxy,
	}
	if cfg.BaseURL == "" {
		log.Warn("MCS_BASE_URL не задан: ссылки в письмах собираются из заголовков запроса, " +
			"за обратным прокси это ненадёжно")
	}

	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           routes(st, apiServer, cfg.StaticDir),
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

// newMailer выбирает способ доставки писем.
//
// Без настроенного SMTP сервис работает, но ссылки уходят в лог — войти
// сможет только тот, у кого есть доступ к логам. Для разработки это удобно,
// для боевого запуска недопустимо, поэтому предупреждение громкое.
func newMailer(cfg config.Config, log *slog.Logger) (mail.Mailer, error) {
	if !cfg.SMTP.Enabled() {
		log.Warn("MCS_SMTP_HOST не задан: письма не отправляются, ссылки выводятся в лог. " +
			"Для боевого запуска настройте почту")
		return mail.NewLogMailer(log), nil
	}

	mailer, err := mail.NewSMTPMailer(mail.SMTPConfig{
		Host:     cfg.SMTP.Host,
		Port:     cfg.SMTP.Port,
		Username: cfg.SMTP.Username,
		Password: cfg.SMTP.Password,
		From:     cfg.SMTP.From,
		Security: mail.Security(cfg.SMTP.Security),
	})
	if err != nil {
		return nil, err
	}

	log.Info("почта настроена", "host", cfg.SMTP.Host, "port", cfg.SMTP.Port,
		"security", cfg.SMTP.Security, "from", cfg.SMTP.From)
	return mailer, nil
}

func routes(st *store.Store, apiServer *api.Server, staticDir string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthz(st))
	apiServer.Register(mux)
	// Всё остальное — оболочка SPA: более длинные шаблоны маршрутов
	// перехватывают свои пути раньше этого.
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
