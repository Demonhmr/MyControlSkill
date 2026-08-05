package config

import "strings"

// Registration — кому разрешено заводить аккаунт.
//
// Ограничивается именно заведение, а не вход: у кого аккаунт уже есть, тот
// входит всегда. Иначе смена списка выбрасывала бы людей посреди пилота, а
// эйчар не мог бы добавить подрядчика с почтой на чужом домене.
type Registration struct {
	// Emails — конкретные адреса.
	Emails map[string]bool
	// Domains — домены целиком, без символа @.
	Domains map[string]bool
}

// Restricted сообщает, задан ли список. Пустой список означает, что завести
// аккаунт может любой, кто знает адрес сервиса.
func (r Registration) Restricted() bool { return len(r.Emails) > 0 || len(r.Domains) > 0 }

// Allows проверяет, разрешено ли адресу заводить аккаунт.
func (r Registration) Allows(email string) bool {
	if !r.Restricted() {
		return true
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if r.Emails[email] {
		return true
	}
	at := strings.LastIndex(email, "@")
	if at < 0 {
		return false
	}
	return r.Domains[email[at+1:]]
}

// parseList разбирает список, разделённый запятыми.
func parseList(raw string, stripAt bool) map[string]bool {
	out := map[string]bool{}
	for _, item := range strings.Split(raw, ",") {
		item = strings.ToLower(strings.TrimSpace(item))
		if stripAt {
			item = strings.TrimPrefix(item, "@")
		}
		if item != "" {
			out[item] = true
		}
	}
	return out
}
