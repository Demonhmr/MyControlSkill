package config

import "testing"

func TestLoadПодставляетУмолчания(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Addr != defaultAddr {
		t.Errorf("Addr = %q, ожидался %q", c.Addr, defaultAddr)
	}
	if c.DBPath != defaultDBPath {
		t.Errorf("DBPath = %q, ожидался %q", c.DBPath, defaultDBPath)
	}
	if c.ShutdownTimeout != defaultShutdownTimeout {
		t.Errorf("ShutdownTimeout = %v", c.ShutdownTimeout)
	}
}

func TestLoadЧитаетОкружение(t *testing.T) {
	t.Setenv("MCS_ADDR", "127.0.0.1:9000")
	t.Setenv("MCS_DB_PATH", "/var/lib/mcs/db.sqlite")
	t.Setenv("MCS_STATIC_DIR", "/srv/static")
	// Хвостовой слэш срезается: ссылки-приглашения собираются конкатенацией.
	t.Setenv("MCS_BASE_URL", "https://compass.example.com/")

	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Addr != "127.0.0.1:9000" {
		t.Errorf("Addr = %q", c.Addr)
	}
	if c.DBPath != "/var/lib/mcs/db.sqlite" {
		t.Errorf("DBPath = %q", c.DBPath)
	}
	if c.StaticDir != "/srv/static" {
		t.Errorf("StaticDir = %q", c.StaticDir)
	}
	if c.BaseURL != "https://compass.example.com" {
		t.Errorf("BaseURL = %q, хвостовой слэш не срезан", c.BaseURL)
	}
}

func TestПустаяПеременнаяЭтоУмолчание(t *testing.T) {
	// В systemd-юните легко получить Environment=MCS_ADDR= — пустое
	// значение не должно валить сервер.
	t.Setenv("MCS_ADDR", "")

	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Addr != defaultAddr {
		t.Errorf("Addr = %q, ожидался %q", c.Addr, defaultAddr)
	}
}

func TestBaseURLБезСхемыОтвергается(t *testing.T) {
	t.Setenv("MCS_BASE_URL", "compass.example.com")

	if _, err := Load(); err == nil {
		t.Fatal("ожидалась ошибка на BaseURL без схемы")
	}
}
