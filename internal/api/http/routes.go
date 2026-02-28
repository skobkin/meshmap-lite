package httpapi

import "net/http"

// Routes returns the API route multiplexer wrapped with request logging.
func (s *Server) Routes(wsHandler http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/healthz", http.HandlerFunc(s.healthz))
	mux.Handle("/readyz", http.HandlerFunc(s.readyz))
	mux.Handle("/api/v1/meta", http.HandlerFunc(s.meta))
	mux.Handle("/api/v1/channels", http.HandlerFunc(s.channels))
	mux.Handle("/api/v1/map/nodes", http.HandlerFunc(s.mapNodes))
	mux.Handle("/api/v1/chat/messages", http.HandlerFunc(s.chatMessages))
	mux.Handle("/api/v1/log/events", http.HandlerFunc(s.logEvents))
	mux.Handle("/api/v1/nodes", http.HandlerFunc(s.nodes))
	mux.Handle("/api/v1/nodes/", http.HandlerFunc(s.nodeByID))
	mux.Handle("/api/v1/ws", wsHandler)

	return s.withRequestLog(mux)
}
