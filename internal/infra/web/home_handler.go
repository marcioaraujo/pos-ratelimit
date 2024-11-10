package web

import "net/http"

type HomeHandler struct {
}

func NewHomeHandler() *HomeHandler {
	return &HomeHandler{}
}

func (h *HomeHandler) Handle(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Hello, World!"))
}
