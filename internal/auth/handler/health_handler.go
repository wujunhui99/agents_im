package handler

import (
	"net/http"

	"github.com/wujunhui99/agents_im/internal/health"
)

func healthHandler(service string) http.HandlerFunc {
	return health.LivenessHandler(service)
}
