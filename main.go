package main

import "net/http"

/*
to build the Server:
go build -o out && ./out
*/

func handler(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
	writer.WriteHeader(http.StatusOK)
	_, err := writer.Write([]byte("OK\n"))
	if err != nil {
		return
	}
}

func main() {
	const filepathRoot = "."
	sMux := http.NewServeMux()
	newServer := http.Server{
		Handler: sMux,
		Addr:    ":8080",
	}
	fServer := http.FileServer(http.Dir(filepathRoot))
	sMux.Handle("/app/", http.StripPrefix("/app", fServer))
	sMux.HandleFunc("/healthz", handler)

	newServer.ListenAndServe()
}
