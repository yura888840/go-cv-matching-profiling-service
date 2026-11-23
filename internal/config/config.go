package config

import (
	"github.com/joho/godotenv"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	DatabaseURL    string
	APIKey         string
	Port           string
	MaxCVs         int
	Workers        int
	AllowedIPs     []string
	AllowedDomains []string
}

func Load() Config {
	_ = godotenv.Load()
	max, _ := strconv.Atoi(os.Getenv("MAX_CVS"))
	if max <= 0 {
		max = 500000
	}
	workers, err := strconv.Atoi(os.Getenv("MATCHING_WORKERS"))
	if os.Getenv("MATCHING_WORKERS") == "" {
		workers = 4
	} else if err != nil || workers < 1 || workers > 64 {
		panic("MATCHING_WORKERS must be an integer from 1 to 64")
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	return Config{DatabaseURL: os.Getenv("DATABASE_URL"), APIKey: os.Getenv("API_KEY"), Port: port, MaxCVs: max, Workers: workers, AllowedIPs: csv(os.Getenv("ALLOWED_IPS")), AllowedDomains: csv(os.Getenv("ALLOWED_DOMAINS"))}
}

func csv(s string) []string {
	var out []string
	for _, v := range strings.Split(s, ",") {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}
