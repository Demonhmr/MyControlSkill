package store

import (
	"errors"
	"testing"
	"time"
)

func TestВходПоСсылкеСоздаётАккаунт(t *testing.T) {
	st, ctx := newTestStore(t)

	if _, err := st.LeaderByEmail(ctx, "new@example.com"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("аккаунт не должен существовать заранее: %v", err)
	}

	token, err := st.CreateLoginToken(ctx, " New@Example.COM ")
	if err != nil {
		t.Fatalf("CreateLoginToken: %v", err)
	}

	leader, err := st.ConsumeLoginToken(ctx, token)
	if err != nil {
		t.Fatalf("ConsumeLoginToken: %v", err)
	}
	if leader.Email != "new@example.com" {
		t.Errorf("почта = %q, ожидалась нормализованная", leader.Email)
	}

	// Второй вход должен попасть в тот же аккаунт, а не создать новый.
	token2, err := st.CreateLoginToken(ctx, "new@example.com")
	if err != nil {
		t.Fatalf("CreateLoginToken: %v", err)
	}
	again, err := st.ConsumeLoginToken(ctx, token2)
	if err != nil {
		t.Fatalf("повторный ConsumeLoginToken: %v", err)
	}
	if again.ID != leader.ID {
		t.Errorf("создан второй аккаунт: %q и %q", leader.ID, again.ID)
	}
}

func TestСсылкаДляВходаОдноразовая(t *testing.T) {
	st, ctx := newTestStore(t)

	token, err := st.CreateLoginToken(ctx, "lead@example.com")
	if err != nil {
		t.Fatalf("CreateLoginToken: %v", err)
	}
	if _, err := st.ConsumeLoginToken(ctx, token); err != nil {
		t.Fatalf("первый вход: %v", err)
	}
	if _, err := st.ConsumeLoginToken(ctx, token); !errors.Is(err, ErrTokenUsed) {
		t.Errorf("повторный вход: ожидался ErrTokenUsed, получено %v", err)
	}
}

func TestПротухшаяСсылкаНеПускает(t *testing.T) {
	st, ctx := newTestStore(t)

	// Письмо могло пролежать в ящике сколько угодно; просроченная ссылка
	// пускать не должна.
	token, err := st.createLoginToken(ctx, "lead@example.com", -time.Minute)
	if err != nil {
		t.Fatalf("createLoginToken: %v", err)
	}
	if _, err := st.ConsumeLoginToken(ctx, token); !errors.Is(err, ErrTokenExpired) {
		t.Errorf("ожидался ErrTokenExpired, получено %v", err)
	}

	// Аккаунт при этом создаваться не должен.
	if _, err := st.LeaderByEmail(ctx, "lead@example.com"); !errors.Is(err, ErrNotFound) {
		t.Errorf("протухшая ссылка создала аккаунт: %v", err)
	}
}

func TestНеизвестнаяСсылкаОтвергается(t *testing.T) {
	st, ctx := newTestStore(t)

	if _, err := st.ConsumeLoginToken(ctx, "выдуманный-токен"); !errors.Is(err, ErrNotFound) {
		t.Errorf("ожидался ErrNotFound, получено %v", err)
	}
}

func TestТокенВходаВБазеНеХранится(t *testing.T) {
	st, ctx := newTestStore(t)

	token, err := st.CreateLoginToken(ctx, "lead@example.com")
	if err != nil {
		t.Fatalf("CreateLoginToken: %v", err)
	}

	var stored string
	if err := st.db.QueryRowContext(ctx, `SELECT token_hash FROM login_token`).Scan(&stored); err != nil {
		t.Fatalf("чтение token_hash: %v", err)
	}
	if stored == token {
		t.Error("в базе лежит сам токен, а не его хэш")
	}
}

func TestТокенСессииВБазеНеХранится(t *testing.T) {
	st, ctx := newTestStore(t)

	leader, err := st.EnsureLeader(ctx, "lead@example.com", "")
	if err != nil {
		t.Fatalf("EnsureLeader: %v", err)
	}
	token, _, err := st.CreateSession(ctx, leader.ID)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	var stored string
	if err := st.db.QueryRowContext(ctx, `SELECT token_hash FROM session`).Scan(&stored); err != nil {
		t.Fatalf("чтение token_hash: %v", err)
	}
	if stored == token {
		t.Error("в базе лежит сам идентификатор сессии, а не его хэш")
	}

	// Значение из базы не должно работать как cookie.
	if _, err := st.LeaderBySession(ctx, stored); !errors.Is(err, ErrNotFound) {
		t.Errorf("хэш из базы сработал как токен сессии: %v", err)
	}
}

func TestСессияЖивётИЗавершается(t *testing.T) {
	st, ctx := newTestStore(t)

	leader, err := st.EnsureLeader(ctx, "lead@example.com", "Пётр")
	if err != nil {
		t.Fatalf("EnsureLeader: %v", err)
	}

	token, session, err := st.CreateSession(ctx, leader.ID)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if session.LeaderID != leader.ID {
		t.Errorf("сессия принадлежит %q, ожидался %q", session.LeaderID, leader.ID)
	}

	got, err := st.LeaderBySession(ctx, token)
	if err != nil {
		t.Fatalf("LeaderBySession: %v", err)
	}
	if got.ID != leader.ID {
		t.Errorf("сессия вернула чужой аккаунт: %q", got.ID)
	}

	if err := st.DeleteSession(ctx, token); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	if _, err := st.LeaderBySession(ctx, token); !errors.Is(err, ErrNotFound) {
		t.Errorf("после выхода ожидался ErrNotFound, получено %v", err)
	}

	// Выход по несуществующей сессии не должен быть ошибкой.
	if err := st.DeleteSession(ctx, "нет-такой"); err != nil {
		t.Errorf("повторный выход: %v", err)
	}
}

func TestПротухшаяСессияНеПускает(t *testing.T) {
	st, ctx := newTestStore(t)

	leader, err := st.EnsureLeader(ctx, "lead@example.com", "")
	if err != nil {
		t.Fatalf("EnsureLeader: %v", err)
	}
	token, _, err := st.createSession(ctx, leader.ID, -time.Second)
	if err != nil {
		t.Fatalf("createSession: %v", err)
	}

	if _, err := st.LeaderBySession(ctx, token); !errors.Is(err, ErrTokenExpired) {
		t.Errorf("ожидался ErrTokenExpired, получено %v", err)
	}
}

func TestУдалениеРуководителяГаситСессии(t *testing.T) {
	st, ctx := newTestStore(t)

	leader, err := st.EnsureLeader(ctx, "lead@example.com", "")
	if err != nil {
		t.Fatalf("EnsureLeader: %v", err)
	}
	token, _, err := st.CreateSession(ctx, leader.ID)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	if _, err := st.db.ExecContext(ctx, `DELETE FROM leader WHERE id = ?`, leader.ID); err != nil {
		t.Fatalf("удаление руководителя: %v", err)
	}
	if _, err := st.LeaderBySession(ctx, token); !errors.Is(err, ErrNotFound) {
		t.Errorf("сессия пережила удаление аккаунта: %v", err)
	}
}

func TestPurgeExpiredAuthЧиститТолькоПротухшее(t *testing.T) {
	st, ctx := newTestStore(t)

	leader, err := st.EnsureLeader(ctx, "lead@example.com", "")
	if err != nil {
		t.Fatalf("EnsureLeader: %v", err)
	}

	liveSession, _, err := st.CreateSession(ctx, leader.ID)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if _, _, err := st.createSession(ctx, leader.ID, -time.Hour); err != nil {
		t.Fatalf("createSession: %v", err)
	}
	liveToken, err := st.CreateLoginToken(ctx, "lead@example.com")
	if err != nil {
		t.Fatalf("CreateLoginToken: %v", err)
	}
	if _, err := st.createLoginToken(ctx, "lead@example.com", -time.Hour); err != nil {
		t.Fatalf("createLoginToken: %v", err)
	}

	if err := st.PurgeExpiredAuth(ctx); err != nil {
		t.Fatalf("PurgeExpiredAuth: %v", err)
	}

	for table, want := range map[string]int{"session": 1, "login_token": 1} {
		var n int
		if err := st.db.QueryRowContext(ctx, `SELECT count(*) FROM `+table).Scan(&n); err != nil {
			t.Fatalf("подсчёт в %s: %v", table, err)
		}
		if n != want {
			t.Errorf("в %s осталось %d строк, ожидалось %d", table, n, want)
		}
	}

	// Живые не пострадали.
	if _, err := st.LeaderBySession(ctx, liveSession); err != nil {
		t.Errorf("живая сессия удалена: %v", err)
	}
	if _, err := st.ConsumeLoginToken(ctx, liveToken); err != nil {
		t.Errorf("живая ссылка удалена: %v", err)
	}
}
