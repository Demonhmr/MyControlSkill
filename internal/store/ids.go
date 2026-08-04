package store

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// newID выдаёт идентификатор записи: 128 бит случайности в hex.
// Внешней зависимости на UUID не заводим — формат нигде не разбирается.
func newID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("генерация идентификатора: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// newToken выдаёт токен приглашения: 256 бит, base64url без выравнивания —
// попадает в ссылку без экранирования.
func newToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("генерация токена: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// hashToken считает то, что попадает в базу вместо самого токена.
//
// Соли и медленной функции здесь нет намеренно: токен — 256 бит из
// crypto/rand, перебирать его нечем, а SHA-256 достаточно, чтобы из дампа
// базы нельзя было восстановить рабочие ссылки.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// timeLayout — формат меток времени в базе.
//
// Дробная часть фиксированной ширины, поэтому лексикографический порядок
// строк совпадает с хронологическим. RFC3339 без неё не годится: секундной
// точности мало, а time.RFC3339Nano отбрасывает хвостовые нули, и тогда
// «…:05.5Z» сортируется раньше «…:05Z» — точка меньше буквы Z.
const timeLayout = "2006-01-02T15:04:05.000Z"

// now — момент времени в том виде, в каком он ложится в базу.
func now() string {
	return time.Now().UTC().Format(timeLayout)
}

// parseTime разбирает временную метку из базы.
//
// Форматов три: основной пишет приложение, RFC3339 без дробной части и
// «2006-01-02 15:04:05» может оставить ручной INSERT со стандартным
// datetime('now'). Молча падать на них не хочется.
func parseTime(s string) (time.Time, error) {
	for _, layout := range []string{timeLayout, time.RFC3339, "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("не разобрана временная метка %q", s)
}

// normalizeEmail приводит почту к виду, по которому ищется аккаунт.
// Уникальный индекс в базе регистра не различает сам по себе, поэтому
// нормализация обязана быть на этой стороне.
func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
