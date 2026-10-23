# webview2

Привязка к WebView2 для Go — копия пакетов `pkg/edge` и `internal/w32` из
[github.com/jchv/go-webview2](https://github.com/jchv/go-webview2)
(лицензия MIT, см. `LICENSE`) с одним изменением: загрузчик WebView2
(`edge/loader.go`) берёт `WebView2Loader.dll` из папки программы, а не из
копии, вшитой в exe и загружаемой из памяти. Из-за такой загрузки
антивирусы принимали программу за вирус.

Сам `WebView2Loader.dll` (подписан Microsoft) лежит в
`third_party/webview2/` и кладётся рядом с exe при сборке.
