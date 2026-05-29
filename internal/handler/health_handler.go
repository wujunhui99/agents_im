package handler

import (
	"net/http"

	"github.com/wujunhui99/agents_im/pkg/health"
)

func healthHandler(service string) http.HandlerFunc {
	return health.LivenessHandler(service)
}
