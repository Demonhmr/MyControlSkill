// Пакет ratelimit ограничивает частоту действий по ключу.
//
// Счётчики держатся в памяти процесса. Для одного экземпляра сервиса этого
// достаточно; если экземпляров станет несколько, лимит окажется у каждого
// свой — тогда понадобится общее хранилище.
package ratelimit

import (
	"sync"
	"time"
)

// Limiter — корзина токенов на каждый ключ.
//
// Корзина, а не счётчик в окне: она допускает короткую серию подряд (человек
// нажал «прислать ссылку» дважды), но держит среднюю частоту, и при этом не
// сбрасывается разом на границе окна.
type Limiter struct {
	// capacity — сколько действий подряд допустимо.
	capacity float64
	// refillPerSecond — с какой скоростью восстанавливается право на действие.
	refillPerSecond float64
	// idleTTL — через сколько неактивности запись выбрасывается.
	idleTTL time.Duration

	mu        sync.Mutex
	buckets   map[string]*bucket
	lastSweep time.Time

	// now подменяется в тестах: ждать реального времени незачем.
	now func() time.Time
}

type bucket struct {
	tokens   float64
	lastSeen time.Time
}

// New создаёт ограничитель: не больше burst действий подряд и не чаще
// limit действий за window в среднем.
func New(limit int, window time.Duration, burst int) *Limiter {
	if limit <= 0 || window <= 0 {
		panic("ratelimit: предел и окно должны быть положительными")
	}
	if burst < 1 {
		burst = 1
	}
	return &Limiter{
		capacity:        float64(burst),
		refillPerSecond: float64(limit) / window.Seconds(),
		// Пустая корзина полностью восстанавливается за capacity/refill
		// секунд; после этого запись ничем не отличается от отсутствующей.
		idleTTL:   window * 2,
		buckets:   make(map[string]*bucket),
		lastSweep: time.Now(),
		now:       time.Now,
	}
}

// Allow списывает одно действие и сообщает, разрешено ли оно. Вторым
// значением — сколько ждать до следующей попытки, если отказано.
func (l *Limiter) Allow(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	l.sweepLocked(now)

	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: l.capacity, lastSeen: now}
		l.buckets[key] = b
	} else {
		elapsed := now.Sub(b.lastSeen).Seconds()
		if elapsed > 0 {
			b.tokens = min(l.capacity, b.tokens+elapsed*l.refillPerSecond)
		}
		b.lastSeen = now
	}

	if b.tokens < 1 {
		missing := 1 - b.tokens
		wait := time.Duration(missing / l.refillPerSecond * float64(time.Second))
		return false, wait
	}
	b.tokens--
	return true, 0
}

// sweepLocked выбрасывает записи, по которым давно не было обращений.
//
// Без этого поток запросов с разными адресами почты раздувал бы карту без
// предела — а именно так и выглядит попытка рассылки.
func (l *Limiter) sweepLocked(now time.Time) {
	const sweepEvery = time.Minute
	if now.Sub(l.lastSweep) < sweepEvery {
		return
	}
	l.lastSweep = now

	for key, b := range l.buckets {
		if now.Sub(b.lastSeen) > l.idleTTL {
			delete(l.buckets, key)
		}
	}
}

// Len — сколько ключей сейчас отслеживается. Нужен тестам и наблюдению.
func (l *Limiter) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.buckets)
}
