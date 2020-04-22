package web

//go:generate forge bindata --ignore .+\.go$ --pkg web -o web_bindata.go -i ./...

func GetIndexPage() ([]byte, error) {
	return indexHtmlBytes()
}
