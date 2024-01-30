package configuration

type Config struct {
	Url       string
	Key       string
	ChunkSize int64
	BarPause  int
	StatusUrl string
	JwtKey    string
}
