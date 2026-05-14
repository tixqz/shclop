package config

type Config struct {
	Addr        string
	Dev         bool
	MockRuntime bool
	MockLLM     bool
	MockSecrets bool
	Store       string
	LogLevel    string
	Metrics     bool
}

func Default() Config {
	return Config{Addr: ":8080", Store: "inmemory", LogLevel: "info", Metrics: true}
}
