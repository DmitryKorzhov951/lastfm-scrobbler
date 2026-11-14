package version

const Name = "Следилка за музыкой"

var Version = "dev"

func UserAgent() string {
	return "lastfm-scrobbler/" + Version + " (+https://github.com/DmitryKorzhov951/cpp_learn)"
}
