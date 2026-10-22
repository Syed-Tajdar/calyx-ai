// Command fake-providers is a LAPTOP-ONLY stand-in for Deepgram and the S3-compatible archive, so the
// whole 2B pipeline can be shown and screenshotted without a real key, a real bucket or a real
// recording. It is not in any image and must never run on the server (production refuses a
// DEEPGRAM_BASE_URL anyway).
//
//	POST /v1/listen                    fake Deepgram: canned diarized utterances (with a card number and an
//	                                   SSN, so the redaction is visible), request id, duration; Spanish when
//	                                   language=multi. It refuses a request without mip_opt_out=true.
//	PUT|HEAD|GET|DELETE /{bucket}/{key}  fake S3, in memory, path-style; auth is not checked
//
// Run with deploy/docker-compose.fake-providers.yml (laptop).
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
)

type store struct {
	mu   sync.Mutex
	objs map[string][]byte
	ct   map[string]string
}

func main() {
	addr := ":8099"
	if v := os.Getenv("FAKE_PROVIDERS_ADDR"); v != "" {
		addr = v
	}
	st := &store{objs: map[string][]byte{}, ct: map[string]string{}}
	http.HandleFunc("/v1/listen", listen)
	http.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	http.HandleFunc("/", st.s3)
	log.Printf("fake-providers (LAPTOP ONLY) listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func listen(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	if q.Get("mip_opt_out") != "true" {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"err_msg":"fake-providers: mip_opt_out=true is required"}`))
		return
	}
	body, _ := io.ReadAll(r.Body)
	sum := sha256.Sum256(body)
	spanish := q.Get("language") == "multi"
	type utt struct {
		Start      float64 `json:"start"`
		End        float64 `json:"end"`
		Speaker    int     `json:"speaker"`
		Transcript string  `json:"transcript"`
	}
	us := []utt{
		{0.4, 3.1, 0, "Thank you for calling Ridge Rise benefits, this is Ana, how can I help you today?"},
		{3.6, 8.2, 1, "Hi, I saw the ad about the final expense plan and I wanted to know the monthly price."},
		{8.9, 15.0, 0, "Of course. For your age the plan starts at $49.99 a month. Can I get your date of birth?"},
		{15.4, 19.7, 1, "Sure, March 3rd 1957. And I can pay with my card, it's 4111 1111 1111 1111."},
		{20.2, 25.9, 0, "Thank you. For the application I also need the last four of your social, or the full number."},
		{26.3, 29.8, 1, "It's 123-45-6789."},
		{30.4, 36.0, 0, "Perfect, you're all set. You'll get the policy documents in 5 to 7 days."},
	}
	lang := "en"
	if spanish {
		lang = "es"
		us = []utt{
			{0.5, 3.4, 0, "Gracias por llamar, mi nombre es Ana, ¿en qué le puedo ayudar?"},
			{3.9, 8.8, 1, "Hola, llamo por el seguro de salud, mi número de seguro social es 5 5 5 1 2 3 4 5 6."},
			{9.2, 14.0, 0, "Muy bien, el plan cuesta $35 al mes. ¿Desea inscribirse hoy?"},
			{14.5, 16.1, 1, "Sí, por favor."},
		}
	}
	resp := map[string]interface{}{
		"metadata": map[string]interface{}{
			"request_id": "fake-" + hex.EncodeToString(sum[:])[:16],
			"duration":   us[len(us)-1].End,
			"models":     []string{"fake-model"},
			"model_info": map[string]interface{}{"fake-model": map[string]string{"name": "general", "version": "fake", "arch": "nova-3"}},
		},
		"results": map[string]interface{}{
			"channels":   []map[string]interface{}{{"detected_language": lang}},
			"utterances": us,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
	log.Printf("fake deepgram: %d bytes, language=%s, redact=%s", len(body), lang, strings.Join(q["redact"], ","))
}

func (s *store) s3(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimPrefix(r.URL.Path, "/")
	if key == "" || !strings.Contains(key, "/") {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	switch r.Method {
	case http.MethodPut:
		b, _ := io.ReadAll(r.Body)
		s.objs[key] = b
		s.ct[key] = r.Header.Get("Content-Type")
		w.Header().Set("ETag", fmt.Sprintf(`"%x"`, sha256.Sum256(b)))
		w.WriteHeader(http.StatusOK)
		log.Printf("fake s3: put %d bytes (sse=%s)", len(b), r.Header.Get("X-Amz-Server-Side-Encryption"))
	case http.MethodHead:
		b, ok := s.objs[key]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Length", fmt.Sprint(len(b)))
		w.WriteHeader(http.StatusOK)
	case http.MethodGet:
		b, ok := s.objs[key]
		if !ok {
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`<?xml version="1.0"?><Error><Code>NoSuchKey</Code></Error>`))
			return
		}
		w.Header().Set("Content-Type", s.ct[key])
		w.Header().Set("Content-Length", fmt.Sprint(len(b)))
		_, _ = w.Write(b)
	case http.MethodDelete:
		delete(s.objs, key)
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
