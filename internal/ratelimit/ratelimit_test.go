package ratelimit

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// withClock подменяет часы: ждать реального времени в тестах незачем.
func withClock(l *Limiter, at *time.Time) *Limiter {
	l.now = func() time.Time { return *at }
	l.lastSweep = *at
	return l
}

func TestСерияПодрядРазрешенаДоПредела(t *testing.T) {
	at := time.Now()
	l := withClock(New(3, time.Minute, 3), &at)

	for i := range 3 {
		if ok, _ := l.Allow("ключ"); !ok {
			t.Fatalf("попытка %d отклонена, а серия должна проходить", i+1)
		}
	}

	ok, wait := l.Allow("ключ")
	if ok {
		t.Fatal("четвёртая попытка подряд прошла")
	}
	if wait <= 0 {
		t.Errorf("не сказано, сколько ждать: %v", wait)
	}
}

func TestПравоВосстанавливаетсяСоВременем(t *testing.T) {
	at := time.Now()
	l := withClock(New(3, time.Minute, 3), &at)

	for range 3 {
		l.Allow("ключ")
	}
	if ok, _ := l.Allow("ключ"); ok {
		t.Fatal("предел не сработал")
	}

	// Три действия в минуту — значит одно право возвращается за 20 секунд.
	at = at.Add(21 * time.Second)
	if ok, _ := l.Allow("ключ"); !ok {
		t.Error("право не восстановилось через 21 секунду")
	}
	if ok, _ := l.Allow("ключ"); ok {
		t.Error("восстановилось больше одного права")
	}
}

func TestКлючиНеВлияютДругНаДруга(t *testing.T) {
	at := time.Now()
	l := withClock(New(1, time.Minute, 1), &at)

	if ok, _ := l.Allow("первый"); !ok {
		t.Fatal("первый ключ отклонён сразу")
	}
	if ok, _ := l.Allow("первый"); ok {
		t.Fatal("предел по первому ключу не сработал")
	}
	// Ограничение по одному адресу не должно затрагивать остальных.
	if ok, _ := l.Allow("второй"); !ok {
		t.Error("второй ключ отклонён из-за первого")
	}
}

func TestСтарыеКлючиВыбрасываются(t *testing.T) {
	at := time.Now()
	l := withClock(New(1, time.Minute, 1), &at)

	// Так выглядит попытка рассылки: поток разных адресов.
	for i := range 500 {
		l.Allow(fmt.Sprintf("адрес-%d@example.com", i))
	}
	if l.Len() != 500 {
		t.Fatalf("ключей в памяти %d, ожидалось 500", l.Len())
	}

	// Спустя два окна неактивности записи ничем не отличаются от
	// отсутствующих и держать их незачем.
	at = at.Add(3 * time.Minute)
	l.Allow("свежий")

	if l.Len() != 1 {
		t.Errorf("после чистки осталось %d ключей, ожидался один", l.Len())
	}
}

func TestЧисткаНеСбрасываетСчётчикАктивногоКлюча(t *testing.T) {
	at := time.Now()
	// Час на три попытки: право возвращается медленно, и за минуту между
	// чистками корзина заведомо не наполнится.
	l := withClock(New(3, time.Hour, 3), &at)

	for range 3 {
		l.Allow("активный")
	}

	// Проходит больше минуты — чистка срабатывает. Ключ при этом свежий,
	// выбрасывать его нельзя: иначе предел обходится паузой в минуту.
	at = at.Add(70 * time.Second)

	if ok, _ := l.Allow("активный"); ok {
		t.Error("счётчик активного ключа сбросился при чистке")
	}
	if l.Len() != 1 {
		t.Errorf("активный ключ выброшен: ключей %d", l.Len())
	}
}

func TestОдновременныеОбращения(t *testing.T) {
	l := New(100, time.Minute, 10)

	var wg sync.WaitGroup
	var mu sync.Mutex
	allowed := 0
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if ok, _ := l.Allow("общий"); ok {
				mu.Lock()
				allowed++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	// Пятьдесят одновременных попыток, запас — десять. Точное число зависит
	// от того, сколько успело накапать, но пропустить все нельзя.
	if allowed > 12 {
		t.Errorf("пропущено %d попыток при запасе 10", allowed)
	}
	if allowed == 0 {
		t.Error("не пропущено ни одной попытки")
	}
}
