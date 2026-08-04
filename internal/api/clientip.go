package api

import (
	"net"
	"net/http"
	"strings"
)

// clientIP определяет адрес, с которого пришёл запрос.
//
// За обратным прокси RemoteAddr — это адрес самого прокси, и все запросы
// выглядят пришедшими с одного места: ограничение по адресу тогда либо
// бессмысленно, либо блокирует всех разом. Помогает X-Forwarded-For, но
// доверять ему без разбора нельзя — клиент присылает этот заголовок сам, и
// подделкой обходится любое ограничение.
//
// Поэтому доверие включается настройкой (MCS_TRUST_PROXY) и берётся
// последнее значение списка: его дописал ближайший прокси, то есть наш.
// Всё, что левее, мог написать клиент.
func (s *Server) clientIP(r *http.Request) string {
	if s.TrustProxy {
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			parts := strings.Split(forwarded, ",")
			last := strings.TrimSpace(parts[len(parts)-1])
			if last != "" {
				return last
			}
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// Без порта — отдаём как есть, лучше грубый ключ, чем никакого.
		return r.RemoteAddr
	}
	return host
}
