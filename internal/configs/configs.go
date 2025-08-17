package configs

import (
	"time"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

type Config struct {
	HTTP struct {
		Host           string
		Port           uint16
		AllowedOrigins []string
		AllowedHeaders []string

		ReadTimeout  time.Duration
		WriteTimeout time.Duration

		TemplateHome       string
		TemplateRoom       string
		TemplateError      string
		LiveReloadTemplate bool

		EnableAccessLog bool
	}
	Rooms struct {
		Capacity       uint
		MemberCapacity uint
	}
}

func Load(path string) *Config {
	var k = koanf.New(".")
	k.Load(file.Provider(path), yaml.Parser())
	cfg := &Config{}

	cfg.HTTP.Host = k.String("http.host")
	cfg.HTTP.Port = uint16(k.Int("http.port"))
	cfg.HTTP.AllowedOrigins = k.MustStrings("http.allowed_origins")
	cfg.HTTP.AllowedHeaders = k.MustStrings("http.allowed_headers")
	cfg.HTTP.ReadTimeout = k.MustDuration("http.read_timeout")
	cfg.HTTP.WriteTimeout = k.MustDuration("http.write_timeout")
	cfg.HTTP.TemplateHome = k.MustString("http.template_home")
	cfg.HTTP.TemplateRoom = k.MustString("http.template_room")
	cfg.HTTP.TemplateError = k.MustString("http.template_error")
	cfg.HTTP.EnableAccessLog = k.Bool("http.enable_access_log")
	cfg.HTTP.LiveReloadTemplate = k.Bool("http.live_reload_template")
	cfg.Rooms.Capacity = uint(k.MustInt("rooms.capacity"))
	cfg.Rooms.MemberCapacity = uint(k.Int("rooms.member_capacity"))

	return cfg
}
