package config

import "testing"

func TestПустойСписокПускаетВсех(t *testing.T) {
	r := Registration{Emails: parseList("", false), Domains: parseList("", true)}

	if r.Restricted() {
		t.Error("пустой список считается ограничивающим")
	}
	if !r.Allows("кто.угодно@example.com") {
		t.Error("пустой список кого-то не пустил")
	}
}

func TestДоменыИАдреса(t *testing.T) {
	r := Registration{
		Emails:  parseList(" Boss@Partner.RU , consultant@other.com ", false),
		Domains: parseList(" @company.ru, Sub.Company.RU ", true),
	}

	if !r.Restricted() {
		t.Fatal("заданный список не считается ограничивающим")
	}

	allowed := []string{
		"lead@company.ru",
		"LEAD@COMPANY.RU",
		"someone@sub.company.ru",
		"boss@partner.ru",
		"consultant@other.com",
	}
	for _, email := range allowed {
		if !r.Allows(email) {
			t.Errorf("адрес %q отклонён", email)
		}
	}

	denied := []string{
		"lead@partner.ru",    // домен целиком не разрешён, только один адрес
		"lead@notcompany.ru", // похожий домен не считается своим
		"lead@company.ru.evil.com",
		"не адрес",
		"",
	}
	for _, email := range denied {
		if r.Allows(email) {
			t.Errorf("адрес %q пропущен", email)
		}
	}
}

func TestСписокЧитаетсяИзОкружения(t *testing.T) {
	t.Setenv("MCS_ALLOWED_DOMAINS", "company.ru, partner.com")
	t.Setenv("MCS_ALLOWED_EMAILS", "boss@other.ru")

	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !c.Registration.Restricted() {
		t.Fatal("список не подхватился")
	}
	if !c.Registration.Allows("lead@partner.com") || !c.Registration.Allows("boss@other.ru") {
		t.Error("разрешённые адреса не проходят")
	}
	if c.Registration.Allows("stranger@example.com") {
		t.Error("посторонний адрес проходит")
	}
}

func TestБезСпискаЗаведениеОткрыто(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// Прежнее поведение сохраняется: сервис, поднятый без настройки, не
	// должен оказаться недоступным сам себе.
	if c.Registration.Restricted() {
		t.Error("без переменных список считается заданным")
	}
	if !c.Registration.Allows("anyone@example.com") {
		t.Error("без списка адрес отклонён")
	}
}
