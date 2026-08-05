package store

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"mycontrolskill/internal/domain"
)

func TestКопияБазыСодержитДанные(t *testing.T) {
	st, ctx := newTestStore(t)
	leader, a := seedAssessment(t, st, ctx)
	inviteAndSubmit(t, st, ctx, a.ID, domain.RolePeer, fullSubmission(domain.TenureOver1Year, 4))

	dest := filepath.Join(t.TempDir(), "копия", "backup.db")
	if err := st.Backup(ctx, dest); err != nil {
		t.Fatalf("Backup: %v", err)
	}

	// Копия должна открываться как обычная база — и уже содержать всё, что
	// было записано, включая изменения из журнала WAL.
	restored, err := Open(context.Background(), dest)
	if err != nil {
		t.Fatalf("копия не открывается: %v", err)
	}
	defer restored.Close()

	got, err := restored.LeaderByID(ctx, leader.ID)
	if err != nil {
		t.Fatalf("руководитель не найден в копии: %v", err)
	}
	if got.Email != leader.Email {
		t.Errorf("почта в копии = %q", got.Email)
	}

	counts, err := restored.CountResponses(ctx, a.ID)
	if err != nil {
		t.Fatalf("подсчёт анкет в копии: %v", err)
	}
	if counts.External != 1 {
		t.Errorf("анкет в копии %d, ожидалась одна", counts.External)
	}
}

func TestКопияНеЗатираетСуществующийФайл(t *testing.T) {
	st, ctx := newTestStore(t)

	dest := filepath.Join(t.TempDir(), "backup.db")
	if err := st.Backup(ctx, dest); err != nil {
		t.Fatalf("первая копия: %v", err)
	}

	// Иначе ежедневный бэкап с одинаковым именем затирал бы вчерашний.
	err := st.Backup(ctx, dest)
	if err == nil {
		t.Fatal("вторая копия перезаписала файл")
	}
	if !strings.Contains(err.Error(), "уже существует") {
		t.Errorf("невнятная причина отказа: %v", err)
	}
}

func TestПустойПутьОтвергается(t *testing.T) {
	st, ctx := newTestStore(t)

	if err := st.Backup(ctx, ""); err == nil {
		t.Error("пустой путь принят")
	}
}

func TestКавычкаВПутиНеЛомаетЗапрос(t *testing.T) {
	st, ctx := newTestStore(t)

	// Путь подставляется в текст запроса, поэтому кавычка в имени каталога
	// не должна превращаться в продолжение SQL.
	dest := filepath.Join(t.TempDir(), "it's a backup.db")
	if err := st.Backup(ctx, dest); err != nil {
		t.Fatalf("Backup с кавычкой в пути: %v", err)
	}

	restored, err := Open(context.Background(), dest)
	if err != nil {
		t.Fatalf("копия не открывается: %v", err)
	}
	restored.Close()
}
