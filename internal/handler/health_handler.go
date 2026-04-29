package handler

import (
	"net/http"

	"github.com/wujunhui99/agents_im/internal/response"
)

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	response.WriteOK(w, map[string]string{"status": "ok"})
}
